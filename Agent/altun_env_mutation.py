import random
import numpy as np

class ElementCycler:
    def __init__(self, lst, step_num):
        self.lst = lst
        self.index = 0  # 
        self.count = 0  # 
        self.step_num = step_num

    def get_element(self):
        element = self.lst[self.index]
        self.count += 1

        if self.count == self.step_num:
            self.count = 0  # reset
            self.index = (self.index + 1) % len(self.lst)  

        return element

def mutate_env(step):
    config = {
        "Concurrency": [i for i in range(100, 1100, 100)],
        "W": [round(i, 1) for i in np.arange(0, 1, 0.1)],
        "K": [i for i in range(1000, 500000, 5000)],
        "buffer_size":[512, 1024, 2048, 4096, 8192, 16384],
        "bft":[False, True]
    }
    
    selected_values = {
        "env_Concurrency": 500,
        "env_W": random.choice(config["W"]),
        "env_K": random.choice(config["K"]),
        "env_buffer_size":random.choice(config["buffer_size"]),
        "env_bft":random.choice(config["bft"])
    }
    
    return selected_values

loop_ploicy_exp_step100 = ElementCycler([True, False], 100)
def constant_env_step100(step):
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_W = 0.9
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft":loop_ploicy_exp_step100.get_element()
    }

loop_ploicy_exp_step20 = ElementCycler([True, False], 20)
def constant_env_step20(step):
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_W = 0.9
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft":loop_ploicy_exp_step20.get_element()
    }

loop_w_env_W = ElementCycler([0.1, 0.5, 0.9], 350)
def loop_w(step):
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_W = [0.1, 0.5, 0.9]
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    
    # # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 350, len(env_W) - 1)
    selected_env_W = loop_w_env_W.get_element() #env_W[index]
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": selected_env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft":False
    }

loop_w_env_W_20 = ElementCycler([round(i, 1) for i in np.arange(0, 1.1, 0.1)], 20)
def loop_w_2(step):
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_W = [round(i, 1) for i in np.arange(0, 1.1, 0.1)]
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
    selected_env_W = loop_w_env_W_20.get_element()
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": selected_env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft":False
    }

loop_w_env_W_100 = ElementCycler([0.1, 0.5, 0.9], 50)
loop_threshold_100 = ElementCycler([3,30,100,500,1000,3000], 20)
distribution_50 = ElementCycler([0,0.5,0.9], 50)
Throttle_50 = ElementCycler([10,100,1000,10000,100000],50)
def loop_new_w(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    Distribution = "uniform"
    nconflict = None
    Throttle = 0
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
    if step < 300 and step > 150 :
        selected_env_W = loop_w_env_W_100.get_element()
    if step >= 300 and step < 420:
        selected_env_t = loop_threshold_100.get_element()
    if step >= 420 and step < 570:
        Distribution = "conflict"
        nconflict = distribution_50.get_element()
    if step >= 570+5*50: 
        Throttle = Throttle_50.get_element()
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": selected_env_W,
        "env_threshold":selected_env_t,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_Distribution": (Distribution,nconflict),
        "env_Throttle":Throttle,
        "env_bft":False
    }

def loop_w_3(step):
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_W = [round(i, 1) for i in np.arange(0, 1.1, 0.1)]
    config = {
        "Concurrency": [i for i in range(100, 1100, 100)],
        "W": [round(i, 1) for i in np.arange(0, 1, 0.1)],
        "K": [i for i in range(1000, 500000, 5000)],
        "buffer_size":[512, 1024, 2048, 4096, 8192, 16384],
        "bft":[False, True]
    }
    
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
    selected_env_W = loop_w_env_W_20.get_element()
    
    return {
         "env_Concurrency": 500,
        "env_K": random.choice(config["K"]),
        "env_buffer_size":random.choice(config["buffer_size"]),
       # "env_bft":random.choice(config["bft"]),
        "env_bft":False,
        "env_W": selected_env_W,
    }


def cft_bft_exp_random(step):
    env_Concurrency = 500
    env_K = 50000
    env_bft = [True, False]
    env_W = 0.1
    env_buffer_size = 1024
    selected_values = {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft":random.choice(env_bft)
    }
    
    return selected_values

loop_cft_bft_exp_step350 = ElementCycler([True, False], 350)
def cft_bft_exp_step350(step):
    env_Concurrency = 500
    env_K = 50000
    env_bft = [True, False]
    env_W = 0.1
    env_buffer_size = 1024
     # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 350, len(env_bft) - 1)
    selected_env_bft = loop_cft_bft_exp_step350.get_element() #env_bft[index]
    selected_values = {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft": selected_env_bft
    }
    
    return selected_values


def random_bft_mutation(step):
    config = {
        "Concurrency": [i for i in range(100, 1100, 100)],
        "W": [round(i, 1) for i in np.arange(0, 1, 0.1)],
        "K": [1000, 10000, 50000, 100000, 500000],
        "buffer_size":[512, 1024, 2048, 4096, 8192, 16384],
        "bft":[False, True],
        "nREPLICAS":[4, 8, 16, 32, 64, 128],
        "nCLIENTS":[1, 10, 100, 500, 1000],
        "nconcurrency":[10, 100, 500, 1000, 2000],
        "nratelimit":[10, 100, 500, 1000, 5000, 10000, 100000],
        "nbatchsize":[1, 10, 100, 500, 1000 ],
        "npayloadsize":[0, 128, 256 ,512, 1024],
        "nleaderrotation":["round-robin", "fixed"]
    }
    
    selected_values = {
        "env_Concurrency":random.choice(config["Concurrency"]),
        "env_W": random.choice(config["W"]),
        "env_K": random.choice(config["K"]),
        "env_buffer_size":random.choice(config["buffer_size"]),
        "env_bft":random.choice(config["bft"]),
        "env_nREPLICAS":random.choice(config["nREPLICAS"]),
        "env_nCLIENTS":random.choice(config["nCLIENTS"]),
        "env_nconcurrency":random.choice(config["nconcurrency"]),
        "env_nratelimit":random.choice(config["nratelimit"]),
        "env_nbatchsize":random.choice(config["nbatchsize"]),
        "env_npayloadsize":random.choice(config["npayloadsize"]),
        "env_nleaderrotation":random.choice(config["nleaderrotation"])
    }
    
    return selected_values

loop_cft_bft_exp_step20 = ElementCycler([True, False], 20)
def cft_bft_exp_step20(step):
    env_Concurrency = 500
    env_K = 50000
    env_bft = [True, False]
    env_W = 0.1
    env_buffer_size = 1024
     # Determine the appropriate value for env_W based on the step value.
    # index = min(step % 20, len(env_bft) - 1)
    selected_env_bft = loop_cft_bft_exp_step20.get_element()
    selected_values = {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft": selected_env_bft
    }
    
    return selected_values

loop_cft_bft_test_step3 = ElementCycler([True, False], 3)
def cft_bft_test_step3(step):
    env_Concurrency = 500
    env_K = 50000
    env_bft = [True, False]
    env_W = 0.9
    env_buffer_size = 1024
     # Determine the appropriate value for env_W based on the step value.
    # index = min(step % 20, len(env_bft) - 1)
    selected_env_bft = loop_cft_bft_test_step3.get_element()
    selected_values = {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft": selected_env_bft
    }
    
    return selected_values


def detection(step):
    env_Concurrency = 500
    env_K = 50000
    env_bft = False
    env_W = 0.9
    env_buffer_size = 1024
    selected_values = {
        "env_Concurrency": env_Concurrency,
        "env_W": env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
        "env_bft": env_bft
    }
    
    return selected_values


# def mutate_W(step):
#     config = {
#         "Concurrency": 500,
#         "W": [round(i, 1) for i in np.arange(0, 1, 0.1)],
#         "K": 50000 ,
#          "env_buffer_size":1024,
#         "bft":False
#     }

#     selected_values = {
#         "env_Concurrency": config["Concurrency"],
#         "env_W": config["W"][step],
#         "env_K": config["K"],
#         "env_buffer_size": config["buffer_size"],
#         "env_bft": config["bft"]
#     }

#     return selected_values

# Function to call reward function by name
def call_env_mutate_function(func_name, step):
    try:
        # Get the function from globals() and call it with the provided arguments
        # print(kwargs)
        reward_func = globals()[func_name]
        return reward_func(step)
    except KeyError:
        raise ValueError(f"Function '{func_name}' is not defined.")
    except TypeError:
        raise ValueError(f"Incorrect arguments provided for function '{func_name}'.")