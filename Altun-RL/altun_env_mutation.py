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
    condition_duration = 300
    condition_index = (step_count // condition_duration) % len(CONDITION_SETS)

    # Get the selected condition set
    selected_conditions = CONDITION_SETS[condition_index]
    logger.info(f"Step {step_count} Total {total_steps}: Progressive mutation to condition set {condition_index + 1}")

    # Create the mutation dictionary with the env_ prefix for each parameter
    mutation_env = {f"env_{k}": v for k, v in selected_conditions.items()}

    # Log the conditions
    logger.info(f"Mutation environment: {mutation_env}")

    return mutation_env


def adaptive_consensus_fast_mutation(step_count):
    """Same CONDITION_SETS cycle as adaptive_consensus_mutation but with
    condition_duration=20 so 120 steps covers all 5 sets twice plus
    a partial third cycle. Used by Exp 2A v2 to trigger cross-family
    switches within a single run.

    5 sets × 20 steps = 100 steps per cycle → 120 steps = 1.2 cycles
    → at least 2 bft-flag transitions (Set 3→4 and Set 5→1).
    """
    condition_duration = 20
    condition_index = (step_count // condition_duration) % len(CONDITION_SETS)
    selected_conditions = CONDITION_SETS[condition_index]
    mutation_env = {f"env_{k}": v for k, v in selected_conditions.items()}
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
# ── Experiment mutations (new) ──────────────────────────────────────────────

# Condition sets from Table 2 (tab:conditions) in the paper
# (nodesize, clients, ratelimit, payloadsize, writeratio, bft)
_BLOCK_SCHEDULE = [
    (4,  4,  5_000,   128,  0.9, False),  # C1 low-load
    (4,  16, 100_000, 1024, 0.9, False),  # C2 high-load
    (8,  16, 10_000,  256,  0.9, False),  # C5 WAN-like
    (4,  16, 100_000, 1024, 0.9, False),  # C2 high-load (return)
    (4,  8,  10_000,  256,  0.9, True),   # C6 safety-alert (bft=True)
    (4,  4,  5_000,   128,  0.9, False),  # C1 low-load (return)
]

def block_schedule_mutation(step, steps_per_block=8):
    """
    Drive the selector through C1→C2→C5→C2→C6→C1, repeated twice (12 blocks).
    Used by Experiment 1 (Selector Justification, Table 5) — v1.
    steps_per_block: steps per block (each step ~45s, so 8 steps ≈ 6 min/block).
    """
    total = len(_BLOCK_SCHEDULE)
    ns, cl, rl, ps, wr, bft = _BLOCK_SCHEDULE[(step // steps_per_block) % total]
    return {
        "env_nodesize":    ns,
        "env_clients":     cl,
        "env_ratelimit":   rl,
        "env_payloadsize": ps,
        "env_writeratio":  wr,
        "env_bft":         bft,
        "env_concurrency": 100,
        "env_batchsize":   1,
    }


def training_set2_fixed_mutation(step):
    """
    Fixed Set 2 conditions matching the adapt_09-15 training window
    [300, 600): the same env parameters DDPG saw between training
    checkpoints 400 and 600. Used for Experiment 1 v2 — DDPG column is
    extracted from training log [500, 596), baselines run live on these
    exact same parameters so the comparison is apples-to-apples on
    workload params (only cluster drift remains).

    All 8 env_* fields are fixed; only the selector's protocol choice
    varies step to step.
    """
    return {
        "env_nodesize":    16,
        "env_clients":     32,
        "env_writeratio":  0.5,
        "env_concurrency": 500,
        "env_ratelimit":   1000,
        "env_batchsize":   1000,
        "env_payloadsize": 256,
        "env_bft":         False,
    }


_DRIFT_BENCHMARK_BLOCKS = [
    # D1: Normal daily operation (vpaxos optimal — 58k, +55% over paxos 38k)
    # Practical: standard workload, moderate clients, moderate rate
    {"env_nodesize":4,  "env_clients":8,  "env_writeratio":0.5,
     "env_concurrency":100, "env_ratelimit":10000, "env_batchsize":1,
     "env_payloadsize":128, "env_bft":False},
    # D2: Traffic spike (paxos optimal — 62k; vpaxos CRASHES at this load)
    # Practical: flash sale / viral event → clients 4× + concurrency 5× + rate 10×
    {"env_nodesize":4,  "env_clients":32, "env_writeratio":0.5,
     "env_concurrency":500, "env_ratelimit":100000, "env_batchsize":1,
     "env_payloadsize":128, "env_bft":False},
    # D3: Scaled-out spike (wpaxos optimal — 16k; vpaxos still crashes at ns=8)
    # Practical: operator scaled cluster 4→8 nodes, spike persists
    {"env_nodesize":8,  "env_clients":32, "env_writeratio":0.5,
     "env_concurrency":500, "env_ratelimit":100000, "env_batchsize":1,
     "env_payloadsize":128, "env_bft":False},
    # D4: BFT security alert (chs/fhs optimal — 72k; CFT→BFT family switch)
    # Practical: Byzantine fault detected, switch to BFT consensus
    {"env_nodesize":4,  "env_clients":16, "env_writeratio":0.5,
     "env_concurrency":500, "env_ratelimit":10000, "env_batchsize":500,
     "env_payloadsize":256, "env_bft":True},
    # D5: Alert cleared + scaled cluster (vpaxos optimal — 28k at ns=16)
    # Practical: back to CFT after security resolved, cluster stays large
    {"env_nodesize":16, "env_clients":8,  "env_writeratio":0.5,
     "env_concurrency":100, "env_ratelimit":10000, "env_batchsize":1,
     "env_payloadsize":128, "env_bft":False},
    # D6: Second spike on small cluster (paxos optimal — repeats D2 pattern)
    # Practical: another traffic spike, proves DDPG can re-adapt not just once
    {"env_nodesize":4,  "env_clients":32, "env_writeratio":0.5,
     "env_concurrency":500, "env_ratelimit":100000, "env_batchsize":1,
     "env_payloadsize":128, "env_bft":False},
]


def drift_benchmark_mutation(step_count, steps_per_block=50):
    """Drift benchmark workload for Figure 5 task-adaptation experiment.

    6 blocks × 50 steps = 300 steps. Scenario C design (Plan C):
      D1 (vpaxos, normal) → D2 (paxos, spike) → D3 (wpaxos, scaled spike)
      → D4 (chs/fhs, BFT alert) → D5 (vpaxos, scaled CFT) → D6 (paxos, 2nd spike)

    4 distinct optimal protocols across 6 blocks. 1 CFT→BFT + 1 BFT→CFT
    family switch. 2 spike events prove repeatability.

    Validated by config-space sweep (324 runs, exp_logs/fig5_sweep_report.md)
    and pre-flight check (_preflight_drift_benchmark.sh).
    """
    block_idx = (step_count // steps_per_block) % len(_DRIFT_BENCHMARK_BLOCKS)
    return dict(_DRIFT_BENCHMARK_BLOCKS[block_idx])


def drift_benchmark_mutation_b20(step_count):
    return drift_benchmark_mutation(step_count, steps_per_block=20)


def drift_benchmark_mutation_b100(step_count):
    return drift_benchmark_mutation(step_count, steps_per_block=100)


def drift_benchmark_mutation_b150(step_count):
    return drift_benchmark_mutation(step_count, steps_per_block=150)


def alternating_regime_mutation(step, regime_steps=20):
    """
    Alternate between two regimes to test thrashing/hysteresis (Table 7).
    Regime A: VPaxos/Paxos-like winner (low concurrency, CFT mode).
    Regime B: FastHotStuff-like winner (high concurrency, BFT mode).
    """
    if (step // regime_steps) % 2 == 0:
        # Regime A
        return {
            "env_nodesize":    4,
            "env_clients":     4,
            "env_concurrency": 100,
            "env_ratelimit":   100_000,
            "env_payloadsize": 64,
            "env_writeratio":  0.9,
            "env_bft":         False,
            "env_batchsize":   1,
        }
    else:
        # Regime B
        return {
            "env_nodesize":    4,
            "env_clients":     16,
            "env_concurrency": 50,
            "env_ratelimit":   50_000,
            "env_payloadsize": 256,
            "env_writeratio":  0.5,
            "env_bft":         True,
            "env_batchsize":   10,
        }

# ── Exp 2A coverage mutation (redesign, 2026-04) ────────────────────────────
# Designed-drift workload that exercises all four family transitions
# (CFT→CFT, CFT→BFT, BFT→CFT, BFT→BFT) by construction. Each of the eight
# phases below is a deliberate workload step; every phase boundary is a
# drift event the agent can (and should) react to. 4 steps per phase ×
# 8 phases × 3 repeats = 96 steps. See
# exp_logs/exp2a_workload_drift_analysis.md §6 P4 for the rationale.
_SWITCH_COVERAGE_SCENARIOS = [
    # --- Scenario A: CFT→CFT (small read-heavy → large write-heavy) ---
    {"env_nodesize": 4,  "env_clients": 4,  "env_concurrency": 50,
     "env_ratelimit": 5_000,   "env_payloadsize": 128, "env_writeratio": 0.1,
     "env_bft": False, "env_batchsize": 1},   # A pre
    {"env_nodesize": 4,  "env_clients": 16, "env_concurrency": 200,
     "env_ratelimit": 100_000, "env_payloadsize": 1024,"env_writeratio": 0.9,
     "env_bft": False, "env_batchsize": 1},   # A post
    # --- Scenario B: CFT→BFT (large CFT → safety alert, env_bft flips) ---
    {"env_nodesize": 4,  "env_clients": 16, "env_concurrency": 200,
     "env_ratelimit": 100_000, "env_payloadsize": 512, "env_writeratio": 0.5,
     "env_bft": False, "env_batchsize": 1},   # B pre
    {"env_nodesize": 4,  "env_clients": 8,  "env_concurrency": 50,
     "env_ratelimit": 10_000,  "env_payloadsize": 256, "env_writeratio": 0.5,
     "env_bft": True,  "env_batchsize": 1},   # B post
    # --- Scenario C: BFT→CFT (post-alert → steady CFT, env_bft flips back) ---
    {"env_nodesize": 4,  "env_clients": 8,  "env_concurrency": 50,
     "env_ratelimit": 10_000,  "env_payloadsize": 256, "env_writeratio": 0.5,
     "env_bft": True,  "env_batchsize": 1},   # C pre
    {"env_nodesize": 4,  "env_clients": 4,  "env_concurrency": 50,
     "env_ratelimit": 5_000,   "env_payloadsize": 128, "env_writeratio": 0.9,
     "env_bft": False, "env_batchsize": 1},   # C post
    # --- Scenario D: BFT→BFT (low-throughput BFT → high-throughput BFT) ---
    {"env_nodesize": 4,  "env_clients": 4,  "env_concurrency": 50,
     "env_ratelimit": 1_000,   "env_payloadsize": 128, "env_writeratio": 0.5,
     "env_bft": True,  "env_batchsize": 1},   # D pre
    {"env_nodesize": 4,  "env_clients": 16, "env_concurrency": 200,
     "env_ratelimit": 10_000,  "env_payloadsize": 256, "env_writeratio": 0.5,
     "env_bft": True,  "env_batchsize": 1},   # D post
]

def switch_coverage_mutation(step, steps_per_phase=4):
    """
    Designed-drift workload for Exp 2A's switch coverage sub-experiment.
    Cycles through 8 workload phases in a fixed order; every phase
    boundary is a known drift event that should cause the agent to
    reconsider its protocol choice. Over 96 steps, each of the 4 family
    transition types (A=CFT→CFT, B=CFT→BFT, C=BFT→CFT, D=BFT→BFT) is
    exercised 3 times, giving statistical power for Table 6.

    Not a replacement for block_schedule_mutation — that one stays
    bit-identical for Exp 2A-natural. This is a new, additive workload
    for the coverage sub-experiment, per exp_logs/
    exp2a_workload_drift_analysis.md §6 P4.
    """
    phase_idx = (step // steps_per_phase) % len(_SWITCH_COVERAGE_SCENARIOS)
    return dict(_SWITCH_COVERAGE_SCENARIOS[phase_idx])


# ── End experiment mutations ─────────────────────────────────────────────────

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