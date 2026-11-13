import numpy as np
import os
from agilerl.components.multi_agent_replay_buffer import MultiAgentReplayBuffer
from agilerl.hpo.mutation import Mutations
from agilerl.hpo.tournament import TournamentSelection
from agilerl.utils.utils import create_population
import torch
import argparse
from maddpg_env import VirtualEnv_MADDPG
from pettingzoo.utils.env import ParallelEnv
from agilerl.algorithms.maddpg import MADDPG
import json

def load_checkpoint(agent, filename):
    if os.path.isfile(filename):
        print(f"Loading checkpoint from {filename}")
        agent.loadCheckpoint(filename)
    else:
        print(f"No checkpoint found at {filename}")

from loguru import logger


def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config_file", type=str, required=True)
    parser.add_argument("--checkpoint", type=str, required=True)
    parser.add_argument(
        "--device",
        type=str,
        default="cuda" if torch.cuda.is_available() else "cpu",
    )
    parser.add_argument("--reward", default="TPS", type=str, choices=["TPS",'latency'])
    parser.add_argument("--reward_fn", type=str, required=True)
    parser.add_argument("--reward_discount", type=float, default=0.8)
    parser.add_argument("--reward_top_num", type=int, default=3)
    parser.add_argument("--record_file", default="log.csv", type=str)
    parser.add_argument("--project_dir", type=str, required=True)    
    parser.add_argument("--saved_config", type=str)

    return parser.parse_args()

import time

if __name__ == "__main__":
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    args=get_args()
    # Get the current time and format it (e.g., "2024-10-10_15-30-00")
    current_time = time.strftime("%Y-%m-%d_%H-%M-%S")
    logger.add(f"{args.saved_folder}/rl_maddpg_{current_time}.log")
    saved_model_path = os.path.join(args.saved_folder, "models")
    os.makedirs(saved_model_path, exist_ok=True)
    # Define the network configuration
    NET_CONFIG = {
        "arch": "mlp",
        "hidden_size": args.hidden_sizes
    }

    INIT_HP = {
        "POPULATION_SIZE": 7,
        "ALGO": "MADDPG",
        "BATCH_SIZE": args.batch_size,
        "O_U_NOISE": True,  # Ornstein Uhlenbeck action noise
        "LR_ACTOR": args.actor_lr,
        "LR_CRITIC": args.critic_lr,
        "GAMMA": args.gamma,
        "MEMORY_SIZE": args.memory_size,
        "LEARN_STEP": args.learn_step,
        "TAU": args.tau,
        "EXPL_NOISE": 0.1,  # Action noise scale
        "MEAN_NOISE": 0.0,  # Mean action noise
        "THETA": 0.15,  # Rate of mean reversion in OU noise
        "POLICY_FREQ": 2,  # Policy frequnecy
        "DT": 0.01,  # Timestep for OU noise
    }

     # Initialize the environment
    env = VirtualEnv_MADDPG(args.config_file, 0, args.reward, args.record_file, args.saved_folder, args)
    env.reset()

    try:
        state_dim = [env.observation_space(agent).n for agent in env.agents]
        one_hot = True
    except Exception:
        state_dim = [env.observation_space(agent).shape for agent in env.agents]
        one_hot = False
    logger.info(f"One hot {one_hot}, state_dim {state_dim}")    
    try:
        action_dim = [env.action_space(agent).n for agent in env.agents]
        INIT_HP["DISCRETE_ACTIONS"] = True
        INIT_HP["MAX_ACTION"] = None
        INIT_HP["MIN_ACTION"] = None
    except Exception:
        for agent in env.agents:
            logger.info(agent)
            logger.info(env.action_space(agent).shape[0])

        action_dim = [env.action_space(agent).shape[0] for agent in env.agents]
        INIT_HP["DISCRETE_ACTIONS"] = False
        INIT_HP["MAX_ACTION"] = [env.action_space(agent).high for agent in env.agents]
        INIT_HP["MIN_ACTION"] = [env.action_space(agent).low for agent in env.agents]

    
    logger.info(f"DISCRETE_ACTIONS {INIT_HP["DISCRETE_ACTIONS"] }")
    logger.info(f"MAX_ACTION {INIT_HP["MAX_ACTION"] }")
    logger.info(f"MIN_ACTION {INIT_HP["MIN_ACTION"] }")
    logger.info(f"env.agents {env.agents} \n action_dim {action_dim}")
    INIT_HP["N_AGENTS"] = env.num_agents
    INIT_HP["AGENT_IDS"] = env.agents



    field_names = ["state", "action", "reward", "next_state", "done"]

    n_agents = env.num_agents
    logger.info(f"num agents {n_agents}")
    agent_ids = [agent_id for agent_id in env.agents]
    max_action = INIT_HP["MAX_ACTION"]
    min_action = INIT_HP["MIN_ACTION"]
    num_envs = 1
    agent = MADDPG(
        state_dims=state_dim,
        action_dims=action_dim,
        one_hot=one_hot,
        n_agents=n_agents,
        agent_ids=agent_ids,
        max_action=max_action,
        min_action=min_action,
        vect_noise_dim=num_envs,
        discrete_actions=INIT_HP["DISCRETE_ACTIONS"],
        device=device,
    )


    agent = agent.load_checkpoint(args.checkpoint)

    agent_reward = {agent_id: 0 for agent_id in env.agents}
    agent_mask = None
    env_defined_actions = (  None )


    logger.info("Iniit System")
    default_config = env.config["default_config_file"]
    # Define different 'W' lists
    W_list = [round(i, 1) for i in np.arange(0, 1, 0.1)]
    # Iterate over W_list and create mutate_env_dict for each
    mutate_env_dict_list = []
    for W in W_list:
        mutate_env_dict = {
            "env_Concurrency": 500,
            "env_W": W,
            "env_K": 50000,
            "env_bft": False
        }

        mutate_env_dict_list.append(mutate_env_dict)

    init_state = env.run_config(default_config, mutate_env=mutate_env_dict_list[0])
    # state = env.get_inference_state()

    max_trial = 10
    for menv in mutate_env_dict_list[1:]:
        for trial in range(10):
            cont_actions, discrete_action = agent.get_action(
                    init_state, False, agent_mask, env_defined_actions
                )
            if agent.discrete_actions:
                    action = discrete_action
            else:
                    action = cont_actions
            mutation_env = None
            applied_config = env.get_new_config(action, mutation_env=mutation_env)
            sys_state = env.run_config(applied_config, mutate_env={})
            new_config_path = args.saved_config
            with open(new_config_path, "w") as f:
                json.dump(applied_config, f)
    

  
     