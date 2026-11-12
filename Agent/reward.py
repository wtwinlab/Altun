

import numpy as np
import heapq
import statistics

# Function to call reward function by name
def call_reward_function(func_name, obs, performance_history, **kwargs):
    try:
        # Get the function from globals() and call it with the provided arguments
        # print(kwargs)
        reward_func = globals()[func_name]
        return reward_func(obs, performance_history, **kwargs)
    except KeyError:
        raise ValueError(f"Function '{func_name}' is not defined.")
    except TypeError:
        raise ValueError(f"Incorrect arguments provided for function '{func_name}'.")
    
def altun_reward(obs, performance_history, k="TPS", discount=0.8, max_top=1):
    # max_top
    if k=='latency':
        mean_top_value = statistics.mean(heapq.nsmallest(max_top, performance_history[k]))
    elif k=='TPS':
        mean_top_value = statistics.mean(heapq.nlargest(max_top, performance_history[k]))
    else:
        assert False, f"Invalid name {k}"
    delta_best = np.sign(obs - mean_top_value) * np.log1p(abs(obs - mean_top_value))
    delta_pre = np.sign(obs - performance_history[k][-1]) * np.log1p(abs(obs - performance_history[k][-1]))
    reward_best = delta_best / (abs(delta_best) + 1)
    reward_pre = delta_pre / (abs(delta_pre) + 1)
    reward = discount * reward_pre + (1 - discount) * reward_best
    return reward
