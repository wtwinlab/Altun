import random
import numpy as np
from loguru import logger
import json
import os

class ElementCycler:
    def __init__(self, lst, step_num):
        self.lst = lst
        self.index = 0  # 当前列表元素的索引
        self.count = 0  # 当前元素的调用次数
        self.step_num = step_num

    def get_element(self):
        element = self.lst[self.index]
        self.count += 1

        # 每step_num次后移动到下一个元素
        if self.count == self.step_num:
            self.count = 0  # 重置计数器
            self.index = (self.index + 1) % len(self.lst)  # 循环访问列表

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

def mutate_env_baseline(step):
    # Save the updated configuration back to the original file
    cun = random.choice([100, 200, 300, 500, 800, 1000, 1200, 1500, 1600, 2000])
    cw = random.choice([0.1, 0.2, 0.3, 0.5, 0.6, 0.7, 0.9])
    selected_values = {
        "env_Concurrency": cun,
        "env_W": cw,
        "env_bft": False
    }
    
    return selected_values

Throttle_20 = ElementCycler([10,100,1000,10000],20)
def comparing_brain_baseline(step):
    # Save the updated configuration back to the original file
    Concurrency = 500 
    W = 0.9
    Throttle = Throttle_20.get_element()
    if step >= 80:
        Throttle = 10000
    selected_values = {
        "env_Concurrency": Concurrency,
        "env_W": W,
        "env_Throttle":Throttle,
        "env_bft": False
    }
    
    
    return selected_values

def mutate_env_baseline_2(step):
    # Save the updated configuration back to the original file
    cun = random.choice([ 500, 800, 1000, 1200, 1500, 1600, 2000])
    cw = random.choice([0.1, 0.2, 0.3, 0.5, 0.6, 0.7, 0.9])
    selected_values = {
        "env_Concurrency": cun,
        "env_W": cw,
        "env_bft": False
    }
    
    return selected_values


def random_mutation(step):
    """
    Mutation function that changes nREPLICAS every 350 steps.
    Keeps other parameters constant to isolate the effect of changing replicas.
    """
    config = {
        "nodesize":[4, 8, 16, 24, 32, 64],
        "clients": [1, 4, 8, 16, 32, 64, 128],
        "concurrency": [4, 100, 200, 500, 1000, 2000, 5000, 10000],
        "batchsize": [1, 10, 100, 500, 1000, 5000, 10000],
        "payloadsize":[64, 128, 256 ,512, 1024, 2048, 4096, 8192, 10240],
        "ratelimit":[100, 500, 1000, 5000, 10000, 100000],
        "writeratio": [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1],
        
    }
    concurrency_rand = random.choice(config["concurrency"])
    batchsize_rand = random.choice(config["batchsize"])
    payloadsize_rand = random.choice(config["payloadsize"])
    ratelimit_rand = random.choice(config["ratelimit"])
    clients_rand = random.choice(config["clients"])
    nodes_rand = random.choice(config["nodesize"])
    writeratio_rand = random.choice(config["writeratio"])
    
    while True:
        concurrency_rand = random.choice(config["concurrency"])
        batchsize_rand = random.choice(config["batchsize"])
        
        if concurrency_rand * 3 > batchsize_rand:
            break  # Exit loop if condition is satisfied

    print(f"Selected values: {nodes_rand}, {clients_rand}, {concurrency_rand}, {writeratio_rand}, {batchsize_rand}, {payloadsize_rand}, {ratelimit_rand}")
    # Return the environment configuration
    return {
        "env_bft": False,
        "env_nodesize": nodes_rand,
        "env_concurrency": concurrency_rand,
        "env_writeratio": writeratio_rand,
        "env_batchsize": batchsize_rand,
        "env_payloadsize": payloadsize_rand,
        "env_ratelimit": ratelimit_rand,
        "env_clients": clients_rand
    }

def random_small_mutation(step):
    """
    Mutation function that changes nREPLICAS every 350 steps.
    Keeps other parameters constant to isolate the effect of changing replicas.
    """
    config = {
        "nodesize":[4, 8, 16],
        "clients": [1, 4, 8, 16, 32],
        "concurrency": [1, 4, 10, 20, 50, 100, 200, 500, 1000],
        "batchsize": [1, 10, 100, 500, 1000, 5000, 10000],
        "payloadsize":[64, 128, 256 ,512, 1024],
        "ratelimit":[100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000],
        # "writeratio": [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1],
        
    }
    concurrency_rand = random.choice(config["concurrency"])
    batchsize_rand = random.choice(config["batchsize"])
    payloadsize_rand = random.choice(config["payloadsize"])
    ratelimit_rand = random.choice(config["ratelimit"])
    clients_rand = random.choice(config["clients"])
    nodes_rand = random.choice(config["nodesize"])
    # writeratio_rand = random.choice(config["writeratio"])
    
    # while True:       
    #     if concurrency_rand * 3 > batchsize_rand:
    #         break  # Exit loop if condition is satisfied
    #     else:
    #         concurrency_rand = random.choice(config["concurrency"])
    #         batchsize_rand = random.choice(config["batchsize"])
        

    print(f"Selected values: {nodes_rand}, {clients_rand}, {concurrency_rand}, {writeratio_rand}, {batchsize_rand}, {payloadsize_rand}, {ratelimit_rand}")
    # Return the environment configuration
    return {
        "env_bft": False,
        "env_nodesize": nodes_rand,
        "env_concurrency": concurrency_rand,
        "env_writeratio": 0.9,
        "env_batchsize": batchsize_rand,
        "env_payloadsize": payloadsize_rand,
        "env_ratelimit": ratelimit_rand,
        "env_clients": clients_rand,
        "env_requests": 1000000
    }

def random_bft_mutation(step):
    """
    Mutation function that changes nREPLICAS every 350 steps.
    Keeps other parameters constant to isolate the effect of changing replicas.
    """
    config = {
        "nodesize":[4, 8, 16],
        "clients": [1, 4, 8, 16, 32],
        "concurrency": [4, 100, 200, 500, 1000],
        "batchsize": [1, 10, 100, 500, 1000, 5000, 10000],
        "payloadsize":[64, 128, 256 ,512, 1024],
        "ratelimit":[200, 500, 1000, 5000, 10000, 100000],
        "writeratio": [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1],
        
    }
    concurrency_rand = random.choice(config["concurrency"])
    batchsize_rand = random.choice(config["batchsize"])
    payloadsize_rand = random.choice(config["payloadsize"])
    ratelimit_rand = random.choice(config["ratelimit"])
    clients_rand = random.choice(config["clients"])
    nodes_rand = random.choice(config["nodesize"])
    writeratio_rand = random.choice(config["writeratio"])
    
    while True:       
        if concurrency_rand * 3 > batchsize_rand:
            break  # Exit loop if condition is satisfied
        else:
            concurrency_rand = random.choice(config["concurrency"])
            batchsize_rand = random.choice(config["batchsize"])
        

    print(f"Selected values: {nodes_rand}, {clients_rand}, {concurrency_rand}, {writeratio_rand}, {batchsize_rand}, {payloadsize_rand}, {ratelimit_rand}")
    # Return the environment configuration
    return {
        "env_bft": True,
        "env_nodesize": nodes_rand,
        "env_concurrency": concurrency_rand,
        "env_writeratio": writeratio_rand,
        "env_batchsize": batchsize_rand,
        "env_payloadsize": payloadsize_rand,
        "env_ratelimit": ratelimit_rand,
        "env_clients": clients_rand
    }

nodes_cycler3 = ElementCycler([4, 8, 16, 24, 32, 64], 20)
def nodesize_bft_rand_mutation(step):
    config = {
        "nodesize":[4, 8, 16],
        "clients": [1, 4, 8, 16, 32],
        "concurrency": [4, 100, 200, 500, 1000],
        "batchsize": [1, 10, 100, 500, 1000, 5000, 10000],
        "payloadsize":[64, 128, 256 ,512, 1024],
        "ratelimit":[200, 500, 1000, 5000, 10000, 100000],
        "writeratio": [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1],
        
    }
    concurrency_rand = random.choice(config["concurrency"])
    batchsize_rand = random.choice(config["batchsize"])
    payloadsize_rand = random.choice(config["payloadsize"])
    ratelimit_rand = random.choice(config["ratelimit"])
    clients_rand = random.choice(config["clients"])
    writeratio_rand = random.choice(config["writeratio"])
    
    while True:       
        if concurrency_rand * 3 > batchsize_rand:
            break  # Exit loop if condition is satisfied
        else:
            concurrency_rand = random.choice(config["concurrency"])
            batchsize_rand = random.choice(config["batchsize"])
        
    
    replica_count = nodes_cycler3.get_element()
    
    print(f"Selected values: {replica_count}, {clients_rand}, {concurrency_rand}, {writeratio_rand}, {batchsize_rand}, {payloadsize_rand}, {ratelimit_rand}")
    # Return the environment configuration
    return {
        "env_bft": True,
        "env_nodesize": replica_count,
        "env_clients": clients_rand,
        "env_concurrency": concurrency_rand,
        "env_writeratio": writeratio_rand,
        "env_ratelimit": ratelimit_rand,
        "env_batchsize":batchsize_rand,
        "env_payloadsize": payloadsize_rand
    }

nodesize_bft_cycler = ElementCycler([4, 8, 12, 16, 24, 32, 48, 64, 96, 128], 20)
def nodesize_bft_mutation(step):
    """
    Mutation function that changes nREPLICAS every 350 steps.
    Keeps other parameters constant to isolate the effect of changing replicas.
    """
    # Get the next replica count from the cycler
    nodesize_rand = nodesize_bft_cycler.get_element()
    
    # Return the environment configuration
    return {
        "env_bft": True,
        "env_nodesize": nodesize_rand,
        "env_clients": 1,
        "env_concurrency": 4,
        "env_ratelimit": 100000,
        "env_writeratio": 0.9,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

nodes_cycler = ElementCycler([4, 8, 12, 16, 24, 32, 48, 64, 96], 20)#128
def nodesize_mutation(step):
    """
    Mutation function that changes nREPLICAS every 350 steps.
    Keeps other parameters constant to isolate the effect of changing replicas.
    """
    
    # Get the next replica count from the cycler
    nodesize_rand = nodes_cycler.get_element()
    
    # Return the environment configuration
    return {
        "env_bft": False,
        "env_nodesize": nodesize_rand,
        "env_clients": 1,
        "env_concurrency": 4,
        "env_ratelimit": 100000,
        "env_writeratio": 0.9,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

clients_cycler = ElementCycler([1, 4, 8, 16, 24, 32, 48, 64, 96, 128], 20)
# [1, 4, 8, 16, 24, 32, 48, 64, 80, 96, 112, 128]
def clients_mutation(step):   
    clients_rand = clients_cycler.get_element()
    
    logger.info(f"!!!!!!! clients_mutation: Selected values: {clients_rand} ")
    return {
        "env_bft": False,
        "env_clients": clients_rand,
        "env_nodesize": 4,
        "env_writeratio": 0.9,
        "env_concurrency": 4,
        "env_ratelimit": 100000,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

concurrency_cycler = ElementCycler([1, 4, 10, 20, 50, 100, 200, 500, 800, 1000], 20)
def concurrency_mutation(step):  
    concurrency_rand = concurrency_cycler.get_element()
    print(f"Selected values: {concurrency_rand} ")
    return {
        "env_concurrency": concurrency_rand,
        "env_bft": False,
        "env_writeratio": 0.9,
        "env_nodesize": 4,
        "env_clients": 1,
        "env_ratelimit": 100000,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

writeratio_cycler = ElementCycler([0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1], 20)
def writeratio_mutation(step):
    writeratio_rand = writeratio_cycler.get_element()
    
    print(f"Selected values: {writeratio_rand} ")
    return {
        "env_bft": False,
        "env_writeratio": writeratio_rand,
        "env_nodesize": 4,
        "env_clients": 1,
        "env_concurrency": 4,
        "env_ratelimit": 100000,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

# [100, 500, 1000, 5000, 10000, 50000, 100000], 50
ratelimit_cycler = ElementCycler([10, 100, 1000, 10000, 100000],20)
# ElementCycler([100, 200, 500, 1000, 2000, 5000, 10000, 50000, 100000, 500000, 1000000], 20)
def ratelimit_mutation(step):
    ratelimit_rand = ratelimit_cycler.get_element()
    if step >= 80:
        ratelimit_rand = 100000

    print(f"Selected values: {ratelimit_rand} ")
    return {
        "env_bft": False,
        "env_ratelimit": ratelimit_rand,
        "env_nodesize": 4,
        "env_clients": 1,
        "env_concurrency": 4,
        "env_writeratio": 0.9,
        "env_batchsize":1,
        "env_payloadsize": 64
    }

# writeratio_cycler1 = ElementCycler([0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1], 20)
payload_cycler = ElementCycler([64, 128, 256 ,512, 1024, 2048, 4096, 8192, 10240], 20)
def payload_mutation(step):
    payload_rand = payload_cycler.get_element()
    # writeratio_rand = writeratio_cycler.get_element()
    print(f"Selected values: {payload_rand} ")
    return {
        "env_bft": False,
        "env_clients": 1,
        "env_nodesize": 4,
        "env_writeratio": 0.9,
        "env_concurrency": 4,
        "env_ratelimit": 100000,
        "env_batchsize":1,
        "env_payloadsize": payload_rand,
    }

batchsize_cycler = ElementCycler([1, 10, 50, 100, 500, 1000, 5000, 10000], 20)
def batchsize_mutation(step):
    batchsize_rand = batchsize_cycler.get_element()
    
    print(f"Selected values: {batchsize_rand} ")
    return {
        "env_bft": False,
        "env_clients": 1,
        "env_nodesize": 4,
        "env_writeratio": 0.9,
        "env_concurrency": 4, # *3
        "env_ratelimit": 100000,
        "env_batchsize":batchsize_rand,
        "env_payloadsize": 64,
    }


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
def loop_new_w(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    Distribution = "uniform"
    nconflict = 0
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


loop_w_env_W_100 = ElementCycler([0.1, 0.5, 0.9], 50)
loop_currency_100 = ElementCycler([500, 1000, 2000], 50)
def loop_new_w_currency(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    Distribution = "uniform"
    nconflict = 0
    Throttle = 0
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
    if step < 150 :
        selected_env_W = loop_w_env_W_100.get_element()
    if step >= 150 :
        env_Concurrency = loop_currency_100.get_element()

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

distribution_50 = ElementCycler([0, 30, 50, 70, 90, 100], 50)
Throttle_50 = ElementCycler([10,100,1000,10000,100000],50)
def loop_new_dist(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    Distribution = "uniform"
    nconflict = 0
    Throttle = 0
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
  
    if step < 300:
        Distribution = "conflict"
        nconflict = distribution_50.get_element()
    if step >= 300: 
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


distribution_comparing = ElementCycler([10, 20, 30, 40, 50, 60, 70, 80, 90], 1)
def comparing_dist(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    nconflict = 0
    Throttle = 0
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
  
    Distribution = "conflict"
    nconflict = distribution_comparing.get_element()
   
    
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


distribution_comparing = ElementCycler([10, 20, 30, 40, 50, 60, 70, 80, 90], 1)
def comparing_dist_stable(step):
    #  ["uniform", "order", "conflict", "normal", "zipfan", "exponential"]
    # if step is in [0, 350], env_w choose 0.1 [350, 700] -> 0.3, and so on.
    env_Concurrency = 500
    env_K = 50000
    env_buffer_size = 1024
    selected_env_W = 0.9
    selected_env_t = 10
    nconflict = 0
    Throttle = 0
    # Determine the appropriate value for env_W based on the step value.
    # index = min(step // 20, len(env_W) - 1)
  
    Distribution = "conflict"
    nconflict = distribution_comparing.get_element()
   
    
    return {
        "env_Concurrency": env_Concurrency,
        "env_W": selected_env_W,
        "env_K": env_K,
        "env_buffer_size":env_buffer_size,
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

# adaptive experiment
CONDITION_SETS = [
    # Condition Set 1: Small-Scale, Read-Heavy Workload
    {
        "nodesize": 4,
        "clients": 8,
        "writeratio": 0.1,
        "concurrency": 200,
        "ratelimit": 100,
        "batchsize": 10,
        "payloadsize": 128,
        "bft": False
    },
    # Condition Set 2: Medium-Scale, Balanced Workload
    {
        "nodesize": 16,
        "clients": 32,
        "writeratio": 0.5,
        "concurrency": 500,
        "ratelimit": 1000,
        "batchsize": 1000,
        "payloadsize": 256,
        "bft": False
    },
    # Condition Set 3: Large-Scale, Write-Heavy Workload
    {
        "nodesize": 32,
        "clients": 64,
        "writeratio": 0.8,
        "concurrency": 1000,
        "ratelimit": 10000,
        "batchsize": 5000,
        "payloadsize": 512,
        "bft": False
    },
    # Condition Set 4: Security-Critical Environment
    {
        "nodesize": 8,
        "clients": 8,
        "writeratio": 0.4,
        "concurrency": 10,
        "ratelimit": 1000,
        "batchsize": 500,
        "payloadsize": 256,
        "bft": True
    },
    # Condition Set 5: High-Throughput, Security-Critical
    {
        "nodesize": 32,
        "clients": 32,
        "writeratio": 0.6,
        "concurrency": 100,
        "ratelimit": 10000,
        "batchsize": 1000,
        "payloadsize": 512,
        "bft": True
    }
]
CONDITION_SETS1 = [
    # Condition Set 1: Small-Scale, Read-Heavy Workload
    {
        "nodesize": 4,
        "clients": 4,
        "writeratio": 0.5,
        "concurrency": 10,
        "ratelimit": 5000,
        "batchsize": 10,
        "payloadsize": 128,
        "bft": False
    },
    # Condition Set 2: Medium-Scale, Balanced Workload
    {
        "nodesize": 8,
        "clients": 8,
        "writeratio": 0.5,
        "concurrency": 100,
        "ratelimit": 10000,
        "batchsize": 100,
        "payloadsize": 256,
        "bft": False
    },
    # Condition Set 3: Large-Scale, Write-Heavy Workload
    {
        "nodesize": 16,
        "clients": 16,
        "writeratio": 0.5,
        "concurrency": 1000,
        "ratelimit": 100000,
        "batchsize": 1000,
        "payloadsize": 512,
        "bft": False
    },
    # Condition Set 4: Security-Critical Environment
    {
        "nodesize": 8,
        "clients": 8,
        "writeratio": 0.4,
        "concurrency": 10,
        "ratelimit": 10000,
        "batchsize": 100,
        "payloadsize": 256,
        "bft": True
    },
    # Condition Set 5: High-Throughput, Security-Critical
    {
        "nodesize": 32,
        "clients": 32,
        "writeratio": 0.6,
        "concurrency": 100,
        "ratelimit": 100000,
        "batchsize": 1000,
        "payloadsize": 512,
        "bft": True
    }
]

CONDITION_SETS_0 = [
    # Condition Set 1: Small-Scale, Read-Heavy Workload
    {
        "nodesize": 4,
        "clients": 8,
        "writeratio": 0.1,
        "concurrency": 200,
        "ratelimit": 5000,
        "batchsize": 10,
        "payloadsize": 128,
        "bft": False
    },
    # Condition Set 2: Medium-Scale, Balanced Workload
    {
        "nodesize": 16,
        "clients": 32,
        "writeratio": 0.5,
        "concurrency": 500,
        "ratelimit": 10000,
        "batchsize": 1000,
        "payloadsize": 256,
        "bft": False
    },
    # Condition Set 3: Large-Scale, Write-Heavy Workload
    {
        "nodesize": 32,
        "clients": 64,
        "writeratio": 0.8,
        "concurrency": 1000,
        "ratelimit": 10000,
        "batchsize": 5000,
        "payloadsize": 512,
        "bft": False
    },
    # Condition Set 4: Security-Critical Environment
    {
        "nodesize": 8,
        "clients": 8,
        "writeratio": 0.4,
        "concurrency": 10,
        "ratelimit": 5000,
        "batchsize": 500,
        "payloadsize": 256,
        "bft": True
    },
    # Condition Set 5: High-Throughput, Security-Critical
    {
        "nodesize": 32,
        "clients": 32,
        "writeratio": 0.6,
        "concurrency": 100,
        "ratelimit": 10000,
        "batchsize": 1000,
        "payloadsize": 512,
        "bft": True
    }
]

def random_consensus_mutation(step_count, condition_duration=200, random_factor=0.1):
    """
    Select environment conditions based on the current step count.
    
    Args:
        step_count: Current step in the training process
        condition_duration: How many steps each condition should last
        random_factor: Probability of introducing a random condition
    
    Returns:
        Dictionary with the mutated environment parameters
    """
    # Decide whether to introduce a random condition
    if np.random.random() < random_factor:
        condition_index = np.random.randint(0, len(CONDITION_SETS))
        logger.info(f"Step {step_count}: Introducing random condition set {condition_index + 1}")
    else:
        # Cycle through condition sets deterministically
        condition_index = (step_count // condition_duration) % len(CONDITION_SETS)
        logger.info(f"Step {step_count}: Using condition set {condition_index + 1}")
    
    # Get the selected condition set
    selected_conditions = CONDITION_SETS[condition_index]
    
    # Create the mutation dictionary with the env_ prefix for each parameter
    mutation_env = {f"env_{k}": v for k, v in selected_conditions.items()}
    
    # Log the conditions
    logger.info(f"Mutation environment: {mutation_env}")
    
    return mutation_env

def adaptive_consensus_mutation(step_count, total_steps=1500):
    # Divide training into segments for each condition set, integer division
    
    # condition_index = min(step_count // segment_length, len(CONDITION_SETS) - 1)
    # Cycle through condition sets deterministically
    # condition_duration = total_steps // len(CONDITION_SETS)
    condition_duration = 50
    condition_index = (step_count // condition_duration) % len(CONDITION_SETS)
        
    # Get the selected condition set
    selected_conditions = CONDITION_SETS[condition_index]
    logger.info(f"Step {step_count} Total {total_steps}: Progressive mutation to condition set {condition_index + 1}")
    
    # Create the mutation dictionary with the env_ prefix for each parameter
    mutation_env = {f"env_{k}": v for k, v in selected_conditions.items()}
    
    # Log the conditions
    logger.info(f"Mutation environment: {mutation_env}")
    
    return mutation_env

def fault_aware_mutation(step_count, fault_detection_file=None, condition_duration=300):
    """
    Adapts conditions based on fault patterns detected during execution.
    
    Args:
        step_count: Current step in the training process
        fault_detection_file: Path to file containing fault information (if any)
        condition_duration: How many steps each condition should last
    
    Returns:
        Dictionary with the mutated environment parameters
    """
    # Base condition is determined by step count
    condition_index = (step_count // condition_duration) % len(CONDITION_SETS)
    selected_conditions = dict(CONDITION_SETS[condition_index])
    
    # Check if fault detection file exists and read it
    detected_faults = False
    if fault_detection_file and os.path.exists(fault_detection_file):
        try:
            with open(fault_detection_file, 'r') as f:
                fault_data = json.load(f)
            
            # Example: If high error rates detected, transition to BFT mode
            if 'error_rate' in fault_data and fault_data['error_rate'] > 0.05:
                selected_conditions['bft'] = True
                detected_faults = True
                logger.warning(f"Detected high error rate: {fault_data['error_rate']}, switching to BFT mode")
        except Exception as e:
            logger.error(f"Error reading fault file: {e}")
    
    # If we detected faults, adjust other parameters as needed
    if detected_faults:
        # More conservative settings when faults are detected
        selected_conditions['ratelimit'] = min(selected_conditions['ratelimit'], 5000)
        selected_conditions['batchsize'] = min(selected_conditions['batchsize'], 500)
    
    logger.info(f"Step {step_count}: Using condition set {condition_index + 1}, fault-aware: {detected_faults}")
    
    # Create the mutation dictionary with the env_ prefix for each parameter
    mutation_env = {f"env_{k}": v for k, v in selected_conditions.items()}
    
    return mutation_env

def workload_specific_mutation(step_count, condition_duration=300):
    """
    Specializes in transitioning between read-heavy, balanced, and write-heavy workloads.
    
    Args:
        step_count: Current step in the training process
        condition_duration: How many steps each condition should last
    
    Returns:
        Dictionary with the mutated environment parameters
    """
    # First determine whether we're in a CFT or BFT phase
    is_bft_phase = (step_count // (condition_duration * 3)) % 2 == 1
    
    # Then determine workload pattern (0=read-heavy, 1=balanced, 2=write-heavy)
    workload_pattern = (step_count // condition_duration) % 3
    
    # Select base conditions based on BFT status and workload pattern
    if is_bft_phase:
        # For BFT, choose between condition sets 4 and 5
        base_conditions = dict(CONDITION_SETS[3 + (workload_pattern // 2)])
        base_conditions['bft'] = True
    else:
        # For CFT, choose between condition sets 1, 2, and 3
        base_conditions = dict(CONDITION_SETS[workload_pattern])
        base_conditions['bft'] = False
    
    # Adjust writer ratio based on workload pattern
    if workload_pattern == 0:  # Read-heavy
        base_conditions['writeratio'] = 0.1
    elif workload_pattern == 1:  # Balanced
        base_conditions['writeratio'] = 0.5
    else:  # Write-heavy
        base_conditions['writeratio'] = 0.8
    
    logger.info(f"Step {step_count}: BFT phase: {is_bft_phase}, Workload: {['read-heavy', 'balanced', 'write-heavy'][workload_pattern]}")
    
    # Create the mutation dictionary with the env_ prefix for each parameter
    mutation_env = {f"env_{k}": v for k, v in base_conditions.items()}
    
    return mutation_env

# Function to call reward function by name
def call_env_mutate_function(func_name, step_count, **kwargs):
    try:
        # Get the function from globals() and call it with the provided arguments
        # print(kwargs)
        mutate_func = globals()[func_name]
        
        # Call the function with step_count and any additional keyword arguments
        return mutate_func(step_count, **kwargs)
    except KeyError:
        raise ValueError(f"Function '{func_name}' is not defined.")
    except TypeError:
        raise ValueError(f"Incorrect arguments provided for function '{func_name}'.")