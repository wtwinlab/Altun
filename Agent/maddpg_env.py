from datetime import datetime
import logging
import json
import os
from glob import glob
from collections import Counter
from gymnasium import spaces
from gymnasium.utils import EzPickle, seeding
import numpy as np
import time
from pettingzoo.utils.env import ParallelEnv
import shutil
from utils import parse_observation_file, update_and_run_config, check_if_run_success,update_config, apply_config, wait_and_delete_file
from loguru import logger
from reward import call_reward_function
from altun_env_mutation import call_env_mutate_function


class VirtualEnv_MADDPG(ParallelEnv, EzPickle):
    """A multi-agent adaptation of the original VirtualEnv for MADDPG."""
    action_count = 0
    step_count = 0
    def __init__(self, config_file, id, reward_model, record_file, saved_folder, args):
        super(VirtualEnv_MADDPG, self).__init__()
        self.id = id
        self.reward_model = reward_model
        self.record_file = record_file
        self._config_action_observation_space(config_file, args)
        self.performance_history = {"TPS": [], "latency": []}
        self.init_info = {"TPS": 0, "latency": 0}
        self.obs_TPS = 0
        self.obs_latency = 0
        self.detection = None
        self.detection_isbft = None
        # this is required by the RL framework
        self.metadata = {
            "render_modes": ["human", "rgb_array"],
            "name": "Fuzzing",
            "render_fps": 60,
        }

        self.args = args
        self.reward_fn = args.reward_fn
        self.reward_top_num = args.reward_top_num
        self.reward_discount = args.reward_discount

        logger.info(f"Write training logger head {self.record_file}")
        
        with open(self.record_file, "w") as f:
            f.write("id,TPS,latency,reward,delta_best,delta_pre,stime,etime,consensus\n")

        # prepare the history_performance_dir and history_action_dir
        self.config['history_performance_dir'] = os.path.join(saved_folder, "performance_history")
        self.config['history_action_dir'] = os.path.join(saved_folder, "history_action")
        logger.info(f"history_performance_dir {self.config['history_performance_dir']}")
        logger.info(f"history_action_dir {self.config['history_action_dir']}")
        if not os.path.isdir(self.config['history_performance_dir']):
            os.makedirs(self.config['history_performance_dir'])
        if not os.path.isdir(self.config['history_action_dir']):
            os.makedirs(self.config['history_action_dir'])

    def _config_action_observation_space(self, config_file, args):
        logger.info(f"Env ID {self.id} start config env, load config from {config_file}")
        self.config = json.load(open(config_file))
        self.config["project_dir"] = args.project_dir
        # Setting up paths for configuration and performance files
        self.config["current_performance_file"] = os.path.join(self.config["project_dir"], self.config["current_performance_file"])
        self.config["default_config_file"] = os.path.join(self.config["project_dir"], self.config["default_config_file"])
        self.config["action_file"] = os.path.join(self.config["project_dir"], self.config["action_file"])
        self.config["env_mutation_file"] = os.path.join(self.config["project_dir"], self.config["env_mutation_file"])
        # 检查文件是否存在
        if os.path.exists(self.config["env_mutation_file"]):
            # 如果文件存在，删除文件
            os.remove(self.config["env_mutation_file"])
            logger.info(f"{self.config["env_mutation_file"]} 已被删除。")
        else:
            logger.info(f"{self.config["env_mutation_file"]} 不存在。")
        # Configure Action Space for multiple agents
        # Set the action space
        actions = self.config["action_space"]
        self.action_space_scale = {}
        self.action_spaces = {}
        self.config["action_agents"] = list(actions['discrete'].keys()) + list(actions['range'].keys())
        logger.info(f"Agent Names {self.config["action_agents"]}")
        for agent_id, agent_name in enumerate(self.config["action_agents"]):
            if agent_name in actions['discrete']:
                self.action_spaces[agent_name] = spaces.Discrete(len(actions['discrete'][agent_name]))
                self.action_space_scale[agent_name] = len(actions['discrete'][agent_name])
                logger.info(f"discrete action {agent_name}, space {self.action_spaces[agent_name].shape},\
                            options {actions['discrete'][agent_name]}, number {len(actions['discrete'][agent_name])}")
            elif agent_name in actions['range']:
                assert False, "Should never be called."


        self.agents = self.config["action_agents"]
        self.possible_agents = self.agents[:] 

        logger.info(f"Agent bft_alg and Agent algorithm cannot be True or False at the same time")
        self.agent_mask = { a:True for a in self.agents }
        bft_in_default = json.load(open(self.config["default_config_file"]))['bft']
        logger.info(f"Default bft {bft_in_default}")
        assert not bft_in_default
        if bft_in_default:
            self.agent_mask['bft_alg'] = True
            self.agent_mask['cft_alg'] = False
        else:
            self.agent_mask['bft_alg'] = False
            self.agent_mask['cft_alg'] = True

        logger.info("Agent Space")
        logger.info(self.action_spaces)
        logger.info(f'Agent Mask {self.agent_mask}')
         # Configure Observation Space
        performance_config =  self.config['observation_space']['performance']
        network_config = self.config['observation_space']['network']
        workload_config =  self.config['observation_space']['workload']
        self.obs_space_scale = {}
        self.observaton_str_mapping_dict = { }

        for n in performance_config:
            self.obs_space_scale[n] = (performance_config[n][0], performance_config[n][1])

        for n in network_config['discrete']:
            self.obs_space_scale[n] = len(network_config['discrete'][n])-1
            self.observaton_str_mapping_dict[n] = {v:i for i,v in enumerate(network_config['discrete'][n])}
        for n in network_config['range']:
            self.obs_space_scale[n] = ( network_config['range'][n][0] , network_config['range'][n][-1] )

        logger.info("Add changed env into the observation space")
        self.env_list_for_change = self.config["env_mutation"]
        for n in self.env_list_for_change:
            if n in workload_config['discrete']:
                # index starts from 0.
                self.obs_space_scale["env_"+n] = len(workload_config['discrete'][n]) - 1
                self.observaton_str_mapping_dict["env_"+n] = {v:i for i,v in enumerate(workload_config['discrete'][n])}
            elif n in workload_config['range']:
                self.obs_space_scale["env_"+n] = ( workload_config['range'][n][0] , workload_config['range'][n][-1] )
            else:
                assert False, f"{n} in self.env_list_for_change is not in worload config"


        for n in workload_config['discrete']:
            self.obs_space_scale[n] = len(workload_config['discrete'][n]) -1
            self.observaton_str_mapping_dict[n] = {v:i for i,v in enumerate(workload_config['discrete'][n])}
            #logger.info(f"workload_config discrete state {n}, options {workload_config['discrete'][n]}, number {len(workload_config['discrete'][n])}")
        for n in workload_config['range']:
            self.obs_space_scale[n] = (workload_config['range'][n][0], workload_config['range'][n][-1]  ) 
            #logger.info(f"workload_config range/continuous action {n}, range {workload_config['range'][n]}")
        
 
        # logger.info(f"observation Space {self.obs_space_scale}")    
        self.space_name = list(self.obs_space_scale.keys())
        observation_space = spaces.Box(low=0, high=1, shape=(len(self.space_name),))
        self.observation_spaces = {agent: observation_space for agent in self.agents}
        for n in performance_config:
            self.obs_space_scale[n] = (performance_config[n][0], performance_config[n][1])
 
        logger.info(f"obs_space_scale: {self.obs_space_scale}")
        logger.info(f"action_space_scale: {self.action_space_scale}")

        # Initialize environment state
        # logger.info("Start Init Env state")
        #if os.path.isfile(self.config["env_mutation_file"]):
        #self.mutation_env = {'env_Concurrency', 'env_W', 'env_K'}
        self.environments =  self.config["env_mutation"]
        init_state_dict = parse_observation_file(self.config['current_performance_file'] if os.path.isfile(self.config['current_performance_file'] ) else None, \
                        self.config['default_config_file'], self.space_name, env_mutation_file=None, environments=self.environments) 
        logger.info("init_state_dict")
        logger.info(init_state_dict)
        self.mutation_env = {}
        for e in self.environments:
            self.mutation_env[f"env_{e}"] = init_state_dict[f"env_{e}"]
        logger.info(f"init mutation env {self.mutation_env}")
        if self.mutation_env['env_bft']:
            self.agent_mask['bft_alg'] = True
            self.agent_mask['cft_alg'] = False
        else:
            self.agent_mask['bft_alg'] = False
            self.agent_mask['cft_alg'] = True

        logger.info(f"observaton_str_mapping_dict")
        logger.info(self.observaton_str_mapping_dict)
        logger.info(init_state_dict)
        for c in self.observaton_str_mapping_dict:
            init_state_dict[c] = self.observaton_str_mapping_dict[c][init_state_dict[c]]
        init_state = []
        # if TPS and latency is not read from self.config['current_performance_file'], set them to zero
        init_state_dict['TPS']= init_state_dict.get('TPS', 0) if init_state_dict.get('TPS', 0) != None else 0
        init_state_dict['latency']=init_state_dict.get('latency', 0) if init_state_dict.get('latency', 0) != None else 0
        for idx, n in enumerate(self.space_name):
            init_state.append(init_state_dict[n] / self.obs_space_scale[n] if  not isinstance(self.obs_space_scale[n],  tuple) else (init_state_dict[n] - self.obs_space_scale[n][0] ) / self.obs_space_scale[n][-1])
        self.init_state = np.asarray(init_state, dtype=np.float32)
        self.init_info = {"TPS":init_state_dict.get('TPS', 0), "latency":init_state_dict.get('latency', 0)}
        # logger.info(f"Finish Init Env state {self.init_state} {self.init_info }")
        logger.info(f"obs_space_scale: {self.obs_space_scale}")
        logger.info(f"action_space_scale: {self.action_space_scale}")

    def step(self, action_dict):
        logger.info(f"ID {self.id} rl step")
        starttime = time.time()
        isSuccess = self._take_action(action_dict)
        terminated = False
        truncated = False

        if isSuccess:
            obs, obs_dic = self._next_observation()
            reward, delta_best, delta_pre = self._cal_reward()
            logger.info(f"reward {reward}, delta_best {delta_best}, reward {delta_pre}")
            endtime=time.time()
            logger.info(f"Execution Action Time {endtime - starttime}")
            algorithm = json.load(open(self.config['action_file']))['algorithm']
            with open(self.record_file, "a") as f:
                f.write(f"{self.action_count}, {obs_dic['TPS']}, {obs_dic['latency']}, {reward}, {delta_best}, {delta_pre},{starttime},{endtime},{algorithm}\n")
            logger.info(f"----- log performance, {obs_dic['TPS']}, {obs_dic['latency']}, {reward}, {delta_best}, {delta_pre},{starttime},{endtime},{algorithm}")
        else:
            logger.info("Rewards are set to 0")
            obs = self.init_state
            reward = 0

        if os.path.isfile(self.config["action_file"]) and isSuccess:
            action_filename = f"action_{self.action_count}.json"
            performance_filename = f"performance_{self.action_count}.json"
            shutil.copyfile(self.config["action_file"], os.path.join(self.config["history_action_dir"], action_filename))
            shutil.copyfile(self.config['current_performance_file'], os.path.join(self.config["history_performance_dir"], performance_filename))
            self.action_count += 1

        #return obs, reward, terminated, truncated, self._get_info()
        # 模拟下一次环境变化
        logger.info(f"Mutation ENV {self.args.mutate_env_fn}")
        self.mutation_env = call_env_mutate_function(self.args.mutate_env_fn, self.step_count)
        logger.info(f"New ENV ============================ {self.mutation_env}")
        self.step_count += 1
        
        #logger.info(f"Detection BFT {self.detection}")
        # if self.detection_isbft is not None:
        #     logger.info(f"Using Detection BFT {self.detection_isbft}")
        #     self.mutation_env['env_bft'] = self.detection_isbft

        if self.mutation_env['env_bft']:
            self.agent_mask['bft_alg'] = True
            self.agent_mask['cft_alg'] = False
            if "bft_pol" in self.agents:
                self.agent_mask['bft_pol'] = True
            if "cft_pol" in self.agents:
                self.agent_mask['cft_pol'] = False
        else:
            self.agent_mask['bft_alg'] = False
            self.agent_mask['cft_alg'] = True
            if "bft_pol" in self.agents:
                self.agent_mask['bft_pol'] = False
            if "cft_pol" in self.agents:
                self.agent_mask['cft_pol'] = True
        logger.info(f"Change ENV {self.mutation_env}")
        logger.info(f"agent_mask {self.agent_mask}")
        with open(self.config['env_mutation_file'], "w") as f:
            json.dump(self.mutation_env, f, indent=2)

        obs = np.asarray([obs.tolist()])

        # update info 
        info = {agent: self._get_info() for agent in self.agents}
        info['agent_mask'] = self.agent_mask
        logger.info(f"env step returned info 2 {info}")
        return {agent: obs for agent in self.agents},\
              {agent: np.asarray([reward]) for agent in self.agents},\
                  {agent: np.asarray([terminated]) for agent in self.agents}, \
                    {agent: np.asarray([truncated]) for agent in self.agents}, info


    def render(self, mode='human'):
        pass
    


    def _next_observation(self):
        logger.info("_next_observation")
        # 获取系统执行action之后的状态
        ob_data = []
        logger.info(self.config['current_performance_file'])
        obs_dic = parse_observation_file(self.config['current_performance_file'], self.config['action_file'], \
                                         self.space_name, env_mutation_file=self.config["env_mutation_file"],\
                                              applied_config_file=self.config["action_file"], action_space_name= self.agents, environments=self.environments) 
        
        detection, detection_isbft = wait_and_delete_file(os.path.join(self.config["project_dir"], "b.env"))
        #logger.info(f"wait_and_delete_file {detection}, {detection_isbft}, {detection is not None}")
        if detection is not None:
            logger.info(f"Detection Changing BFT {detection_isbft}")
            self.detection_isbft = detection_isbft
            obs_dic['env_bft'] = self.detection_isbft

        self.performance_history['TPS'].append(self.obs_TPS)
        self.performance_history['latency'].append(self.obs_latency)
        

        self.obs_TPS = obs_dic["TPS"] #( obs_dic["TPS"] - self.obs_space_scale["TPS"][0] ) / self.obs_space_scale["TPS"][1]
        self.obs_latency =  obs_dic["latency"]  #( obs_dic["latency"] - self.obs_space_scale["latency"][0] ) / self.obs_space_scale["latency"][1]
        for c in self.observaton_str_mapping_dict:
            logger.info(obs_dic[c])
            logger.info(self.observaton_str_mapping_dict[c])
            obs_dic[c] = self.observaton_str_mapping_dict[c][obs_dic[c]]
        # 把数据归一化，范围， 0-1
        for idx, n in enumerate(self.space_name):
            # print(obs_dic)
            ob_data.append( obs_dic[n] / self.obs_space_scale[n] if  not isinstance(self.obs_space_scale[n],  tuple) else (obs_dic[n] - self.obs_space_scale[n][0] ) / ( self.obs_space_scale[n][-1] - self.obs_space_scale[n][0]))
        
        obs_state = np.asanyarray(ob_data)
        self.init_state = obs_state
        self.init_info = {"TPS":obs_dic["TPS"], "latency":obs_dic["latency"]}
        return obs_state, obs_dic

  

    def _take_action(self, action_dict):
        action_explain = {}
        logger.info("Action")
        logger.info(action_dict)
        for agent_name, action_value in action_dict.items():
            #logger.info(action_value)
            scale = self.action_space_scale[agent_name]

            if isinstance(scale, tuple):  # Continuous action
                assert False, "Should Never be called"
                action_value = action_value[0][0]
                action_explain[agent_name] = int(np.round(float(action_value) * (scale[1] - scale[0]) + scale[0])) # (int(action_value) + scale[0])*scale[-1]
            else:  # Discrete action
                action_value = action_value[0]
                # logger.info(f"np.round(action_value*scale) {np.round(action_value*scale)}")
                action_explain[agent_name] = int(action_value)
                logger.info(f"Idx Discrete {agent_name}: {action_explain[agent_name]}")
                action_explain[agent_name]  = self.config["action_space"]["discrete"][agent_name][action_explain[agent_name]]
            logger.info(f"{agent_name}, scale {scale}, action value {action_value} -> action_explain {action_explain[agent_name]}")
        logger.info(f"action_explain {action_explain}")

        if  self.agent_mask['bft_alg'] and self.agent_mask['cft_alg']:
            assert False, "bft_alg and algorithm of cft cannot be True at the same time"
        
        if self.agent_mask['bft_alg'] != self.mutation_env["env_bft"]:
            assert False, "BFT is enabled and bft agent mask should be True. BFT is disabled and bft agent mask should be False."
        
        if self.mutation_env["env_bft"] and  self.agent_mask['cft_alg']:
            assert False, "BFT is enabled and cft agent mask should be False."
        
        if not self.mutation_env["env_bft"] and  not self.agent_mask['cft_alg']:
            assert False, "BFT is disabled and cft agent mask should be True."
        
        action_updated_config = {}
        if self.agent_mask['bft_alg']:
            action_updated_config['algorithm'] = action_explain['bft_alg']
            if "bft_pol" in action_explain:
                action_updated_config['policy'] = action_explain['bft_pol']
        else:
            action_updated_config['algorithm'] = action_explain['cft_alg']
            if "cft_pol" in action_explain:
                action_updated_config['policy'] = action_explain['cft_pol']

        
        for a in self.agents:
            if a not in ['bft_alg', 'cft_alg', 'cft_pol', 'bft_pol']:
                config_name = a.replace("_agent", "")
                action_updated_config[config_name] = action_explain[a]

        logger.info(f"applied action config {action_updated_config}, agent mask {self.agent_mask}")
        logger.info(f"Environemt Change {self.step_count} {self.mutation_env}")
        update_and_run_config(self.config['action_file'], self.config['action_file'], \
                              action_updated_config, os.path.join(self.config['project_dir'], "history_configs"), \
                              self.args.project_dir, self.config["action_space"]["discrete"], mutation_env=self.mutation_env,
                               threshold_time= self.config['run_timeout'], max_retries= self.config['run_trial_num'])
        try:
            isSuccess, parsed_performance, raw_performace = check_if_run_success(self.config['current_performance_file'])
            logger.info(f"Run {isSuccess}, Parsed {parsed_performance}, Raw Performance {raw_performace}")
        except Exception as e:
            logger.error(f"Run Config Failed with error: {e}")
            isSuccess = False
        return isSuccess

    def _cal_reward(self):
        if len(self.performance_history["TPS"]) == 0 or self.performance_history["TPS"][-1] == 0:
            return 0, 0, 0
        else:
            if self.reward_model=="TPS":
                logger.info("Computing TPS reward")
                delta_best = self.obs_TPS - max(self.performance_history["TPS"])
                delta_pre = self.obs_TPS - self.performance_history["TPS"][-1]
                reward = call_reward_function(self.reward_fn, self.obs_TPS, self.performance_history, k="TPS",\
                                                discount=self.reward_discount, max_top=self.reward_top_num)
            if self.reward_model=='latency':
                logger.info("Computing latency reward")
                delta_best = self.obs_latency - max(self.performance_history["latency"])
                delta_pre = self.obs_latency - self.performance_history["latency"][-1]
                reward = -call_reward_function(self.reward_fn, self.obs_latency, self.performance_history, k="latency",\
                                                discount=self.reward_discount, max_top=self.reward_top_num)
            
            if reward < 0:
                reward = 0

            return reward, delta_best, delta_pre

    def _get_info(self):
        return self.init_info


    def reset(self, seed=None, options=None):
        # super().reset(seed=seed)
        self.previous_iq = 0
        self.agents = self.possible_agents[:]
        self.terminations = {agent: False for agent in self.agents}

        obs = np.asarray([self.init_state.tolist()])
        infos = {agent: {} for agent in self.agents}
        infos['agent_mask'] = self.agent_mask
        # with open(self.record_file, "a") as f:
        #         f.write(f"reset\n")
        return {agent: obs for agent in self.agents}, infos
    
    def observation_space(self, agent):
        return self.observation_spaces[agent]
    
    def action_space(self, agent):
        return self.action_spaces[agent]
    


        
        

    
