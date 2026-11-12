import os
from agilerl.components.multi_agent_replay_buffer import MultiAgentReplayBuffer
from agilerl.hpo.tournament import TournamentSelection
from agilerl.utils.utils import create_population
import torch
import argparse
from maddpg_env import VirtualEnv_MADDPG
from agilerl.algorithms.maddpg import MADDPG
from loguru import logger
import time
import re
import json
import pdb
def load_checkpoint(agent, filename):
    if os.path.isfile(filename):
        print(f"Loading checkpoint from {filename}")
        agent.loadCheckpoint(filename)
    else:
        print(f"No checkpoint found at {filename}")



def get_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--memory_size", type=int, default=10000)                     # 
    parser.add_argument("--hidden_sizes", type=int, nargs="*", default=[128, 128])  #
    parser.add_argument("--actor_lr", type=float, default=1e-3)                     #
    parser.add_argument("--critic_lr", type=float, default=1e-2)                    #
    parser.add_argument("--gamma", type=float, default=0.99)                        #
    parser.add_argument("--tau", type=float, default=0.001)                         #
    parser.add_argument("--max_steps", type=int, default=150)                       #
    parser.add_argument("--learn_step", type=int, default=5)                        #
    parser.add_argument("--batch_size", type=int, default=16)                       #
    parser.add_argument("--saved_folder", type=str, required=True)
    parser.add_argument("--render", type=float, default=0.0)
    parser.add_argument("--config_file", type=str, required=True)
    parser.add_argument(
        "--device",
        type=str,
        default="cuda" if torch.cuda.is_available() else "cpu",
    )
    parser.add_argument("--reward", default="TPS", type=str, choices=["TPS",'latency'])
    # parser.add_argument("--reward_fn", type=str, required=True)
    # Updated reward function options
    parser.add_argument(
        "--reward_fn", 
        type=str, 
        default="best_and_pre",
        choices=[
            "best_and_pre",  # Original function
            "adaptive_consensus_reward", # experiment
            "dynamic_reward_function",        # New function 1
            "adaptive_reward_scaling",        # New function 2
            "env_aware_reward",              # New function 3
            "progressive_reward_shaping",     # New function 4
            "multi_objective_reward"         # New function 5
        ],
        help="Reward function to use during training"
    )
    parser.add_argument("--mutate_env_fn", type=str, default="mutate_env")
    parser.add_argument("--reward_discount", type=float, default=0.8)
    parser.add_argument("--reward_top_num", type=int, default=3)
    parser.add_argument("--record_file", default="log.csv", type=str)
    parser.add_argument("--project_dir", type=str, required=True)    
    parser.add_argument("--saved_steps", type=int, default=20)
    parser.add_argument("--inference",  action='store_true')
    parser.add_argument("--checkpoint", type=str, default=None)
    # parameters for advanced reward functions
    parser.add_argument("--window_size", type=int, default=20,
                      help="Window size for adaptive reward scaling")
    parser.add_argument("--total_steps", type=int, default=5000,
                      help="Total expected steps for progressive reward shaping")
    # Hysteresis parameters for switch cost / thrashing experiments
    parser.add_argument("--cooldown_steps", type=int, default=0,
                      help="Min steps between protocol switches (0=disabled)")
    parser.add_argument("--switch_margin", type=float, default=0.0,
                      help="Min relative TPS improvement to allow a switch (0=disabled)")
    # Controller mode: use altun-controller API instead of kill/restart
    parser.add_argument("--controller_addr", type=str, default=None,
                      help="altun-controller address (e.g. localhost:8080). "
                           "If set, protocol switches go through the controller "
                           "gRPC/HTTP API instead of run_system.sh.")

    return parser.parse_args()



if __name__ == "__main__":
    # check if GPU is avaiable or not.
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    # read passed parameters
    args=get_args()
    # Get the current time and format it (e.g., "2024-10-10_15-30-00")
    current_time = time.strftime("%Y-%m-%d_%H-%M-%S")
    # create a log file name with f"{args.saved_folder}/rl_maddpg_{current_time}.log"
    logger.add(f"{args.saved_folder}/rl_maddpg_{current_time}.log")
    logger.info(f"===== Altun MADDPG, Training {not args.inference} {args.hidden_sizes}=====")
    # save Agent weights to args.saved_folder/"models"
    saved_model_path = os.path.join(args.saved_folder, "models")
    os.makedirs(saved_model_path, exist_ok=True)
    # Define the network/deep learning network configuration
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
    # breakpoint()
    env = VirtualEnv_MADDPG(args.config_file, 0, args.reward, args.record_file, args.saved_folder, args)
    # env = parallel_to_aec_wrapper(base_env)  # Convert to AEC format if needed
    # env = parallel_wrapper_fn(env)           # Apply parallel wrapper
    # env = env()
    
    # set the environemtn to the inital state
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
        for maddpg_agent in env.agents:
            logger.info(maddpg_agent)
            logger.info(env.action_space(maddpg_agent))

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

    pop = create_population(
        INIT_HP["ALGO"],
        state_dim,
        action_dim,
        one_hot,
        NET_CONFIG,
        INIT_HP,
        population_size=INIT_HP["POPULATION_SIZE"],
        num_envs=1,
        device=device,
    )

    # Load saved checkpoints if available
    # load_step = 1700

    field_names = ["state", "action", "reward", "next_state", "done"]
    memory = MultiAgentReplayBuffer(
        INIT_HP["MEMORY_SIZE"],
        field_names=field_names,
        agent_ids=INIT_HP["AGENT_IDS"],
        device=device,
    )

    tournament = TournamentSelection(
        tournament_size=2,
        elitism=True,
        population_size=INIT_HP["POPULATION_SIZE"],
        eval_loop=1,
    )

 
    n_agents = env.num_agents
    logger.info(f"num agents {n_agents}")
    agent_ids = [agent_id for agent_id in env.agents]
    max_action = INIT_HP["MAX_ACTION"]
    min_action = INIT_HP["MIN_ACTION"]
    num_envs = 1
    maddpg_agent = MADDPG(
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

    if args.inference and not os.path.isfile(args.checkpoint):
        logger.error(f"Please Provide checkpoint for the model for inference, args.inference {args.inference}, checkpoint {args.checkpoint}")
        assert False, f"Please Provide checkpoint for the model for inference, args.inference {args.inference}, checkpoint {args.checkpoint}"

    base_number = 0
    if args.checkpoint is not None and os.path.isfile(args.checkpoint):
        logger.info(f"Load Checkpoint {args.checkpoint}")
        maddpg_agent.load_checkpoint(args.checkpoint)
        # 使用正则表达式提取所有数字
        cname = os.path.basename(args.checkpoint)
        numbers = re.findall(r'\d+', cname)

        # 如果需要将提取到的数字转为整数，可以这样做
        numbers = [int(num) for num in numbers]
        base_number = numbers[0]

    # max_episodes = args.max_episodes
    max_steps = args.max_steps
    epsilon = 1.0
    eps_end = 0.1
    eps_decay = 0.995
    evo_epochs = 20
    evo_loop = 1
    elite = pop[0]

    state, info = env.reset()
    # logger.info(f"state {state}")
    logger.info(f"Training Init info {info}")
    agent_reward = {agent_id: 0 for agent_id in env.agents}
    if args.inference:
        is_training = False
    else:
        is_training = True
    logger.info(f"Training {is_training}")
    execution_times = [ ]
    timestamp_file = [ ]
    for step in range(max_steps):
        agent_mask = info["agent_mask"] if "agent_mask" in info.keys() else None
        env_defined_actions = (
            info["env_defined_actions"]
            if "env_defined_actions" in info.keys()
            else None
        )
        logger.info(f"Training Agen Mask {agent_mask}")
        astart_time = time.time()
        cont_actions, discrete_action = maddpg_agent.get_action(
            state, is_training, info
        )
        aend_time = time.time()
        t = aend_time - astart_time
        logger.info(f"Get Action Time Detla {t}, Timestamp {astart_time} - {aend_time}")
        execution_times.append(t)
        
        if maddpg_agent.discrete_actions:
            action = discrete_action
        else:
            action = cont_actions
        logger.info(f"Training get action {action}")
        sstart_time = time.time()
        next_state, reward, termination, truncation, info = env.step(action)
        send_time = time.time()
        logger.info(f"Step Time {send_time - sstart_time}")
        timestamp_file.append({"action_start":astart_time, "action_end":aend_time, "execute_start":sstart_time, "execute_end":send_time})
        # logger.info(f"next_state {next_state}")
        logger.info(f"reward {reward}")
        # logger.info("+++++++++++++++++++++++++1")
        if is_training:    
            memory.save_to_memory(state, cont_actions, reward, next_state, termination,  is_vectorised=True)

        for agent_id, r in reward.items():
            agent_reward[agent_id] += r[0]

        if (memory.counter % maddpg_agent.learn_step == 0) and (
            len(memory) >= maddpg_agent.batch_size
        ) and is_training:
            experiences = memory.sample(maddpg_agent.batch_size)
            logger.info("Learning, Update Agent Model Weights")
            maddpg_agent.learn(experiences)

        if step % 50:
            json.dump(timestamp_file, open(os.path.join(args.saved_folder, "timestampe.json"),  "w"))
        state = next_state

        if any(truncation.values()) or any(termination.values()):
            break

        if memory.counter % args.saved_steps == 0 and is_training:
            filename = f"MADDPG_trained_agent_{memory.counter+base_number}.pt"
            checkpoint_path = os.path.join(saved_model_path, filename)
            maddpg_agent.save_checkpoint(checkpoint_path)
    # Compute the average execution time
    average_execution_time = sum(execution_times) / len(execution_times)
    logger.info(f'Average execution time per iteration: {sum(execution_times)}/{len(execution_times)} {average_execution_time:.4f} seconds')
    json.dump(timestamp_file, open(os.path.join(args.saved_folder, "timestampe.json"), "w"))
    if is_training:
        logger.info("Save Final Model Weights")
        filename = f"MADDPG_trained_agent_{memory.counter+base_number}_final.pt"
        checkpoint_path = os.path.join(saved_model_path, filename)
        maddpg_agent.save_checkpoint(checkpoint_path)
