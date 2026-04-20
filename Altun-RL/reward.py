import numpy as np
import heapq
import statistics
import json
import os
from loguru import logger

# Function to call reward function by name
def call_reward_function(func_name, obs, performance_history, **kwargs):
    try:
        # Get the function from globals() and call it with the provided arguments
        reward_func = globals()[func_name]
        # Get the parameters that the reward function accepts
        import inspect
        params = inspect.signature(reward_func).parameters
        param_names = list(params.keys())
        
        # Filter kwargs to only include parameters that the function accepts
        filtered_kwargs = {k: v for k, v in kwargs.items() if k in param_names}
        
        # Call the function with appropriate arguments
        return reward_func(obs, performance_history, **filtered_kwargs)
    except KeyError:
        raise ValueError(f"Function '{func_name}' is not defined.")
    except TypeError:
        raise ValueError(f"Incorrect arguments provided for function '{func_name}'.")
    
def best_and_pre(obs, performance_history, k="TPS", discount=0.8, max_top=1):
    # 计算性能历史中最小的 max_top 个延迟的平均值
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

# New reward functions

def dynamic_reward_function(obs, performance_history, step_count=0, k="TPS", **kwargs):
    """
    Dynamically adjusts the reward function based on training progress.
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        step_count: Current training step count
        k: Key for performance metric ("TPS" or "latency")
        **kwargs: Additional keyword arguments
        
    Returns:
        Calculated reward
    """
    # Early training: Focus more on immediate improvements: 1000, 3000
    if step_count < 500:
        discount = 0.9  # Higher weight for previous performance comparison
    # Mid training: Balance between immediate and best historical performance
    elif step_count < 1000:
        discount = 0.7
    # Late training: Focus more on improving over best historical performance
    else:
        discount = 0.5
    
    return best_and_pre(obs, performance_history, k=k, discount=discount, **kwargs)

def adaptive_reward_scaling(obs, performance_history, k="TPS", window_size=20, **kwargs):
    """
    Adapts reward scaling based on the variance of recent performance.
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        k: Key for performance metric ("TPS" or "latency")
        window_size: Number of recent performances to consider
        **kwargs: Additional keyword arguments
        
    Returns:
        Calculated reward with adaptive scaling
    """
    # Get recent performance window
    recent_perf = performance_history[k][-window_size:] if len(performance_history[k]) > window_size else performance_history[k]
    
    # Calculate variance of recent performance to determine stability
    variance = np.var(recent_perf) if len(recent_perf) > 1 else 1.0
    
    # Scale the discount factor based on variance
    # High variance (unstable) -> higher discount (focus on immediate improvements)
    # Low variance (stable) -> lower discount (focus on best historical performance)
    normalized_variance = min(1.0, variance / (np.mean(recent_perf) + 1e-8))
    adaptive_discount = 0.5 + (0.4 * normalized_variance)
    
    # Use the adaptive discount in the reward calculation
    return best_and_pre(obs, performance_history, k=k, discount=adaptive_discount, **kwargs)

def env_aware_reward(obs, performance_history, k="TPS", is_bft_mode=False, **kwargs):
    """
    Adjusts rewards based on the current environment (BFT or CFT mode).
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        k: Key for performance metric ("TPS" or "latency")
        is_bft_mode: Flag indicating if the system is in BFT mode
        **kwargs: Additional keyword arguments
        
    Returns:
        Environment-aware calculated reward
    """
    # Different reward scaling for different consensus modes
    if is_bft_mode:
        # BFT mode may have different performance expectations
        max_top = 3  # Compare with top 3 historical performances
        discount = 0.7  # More emphasis on historical best
    else:
        # CFT mode
        max_top = 1  # Compare only with the best historical performance
        discount = 0.8  # More emphasis on immediate improvement
        
    return best_and_pre(obs, performance_history, k=k, 
                                         discount=discount, max_top=max_top, **kwargs)

def progressive_reward_shaping(obs, performance_history, step_count=0, total_steps=5000, k="TPS", **kwargs):
    """
    Progressively changes the reward function shape during training.
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        step_count: Current training step count
        total_steps: Total number of training steps
        k: Key for performance metric ("TPS" or "latency")
        **kwargs: Additional keyword arguments
        
    Returns:
        Progressively shaped reward
    """
    # Calculate training progress (0 to 1)
    progress = min(1.0, step_count / total_steps)
    
    # Early training: Use simpler reward function focusing on immediate improvements
    if progress < 0.3:
        delta_pre = np.sign(obs - performance_history[k][-1]) * np.log1p(abs(obs - performance_history[k][-1]))
        reward_pre = delta_pre / (abs(delta_pre) + 1)
        return reward_pre
    
    # Mid to late training: Gradually shift to more sophisticated reward
    else:
        # Dynamically adjust max_top based on progress
        max_top = max(1, int(1 + progress * 5))  # Increases from 1 to 6
        
        # Dynamically adjust discount factor
        discount = max(0.5, 0.9 - (progress * 0.4))  # Decreases from 0.9 to 0.5
        
        return best_and_pre(obs, performance_history, k=k, 
                                             discount=discount, max_top=max_top, **kwargs)

def multi_objective_reward(obs, performance_history, step_count=0, k="TPS", **kwargs):
    """
    Combines multiple reward objectives with dynamic weighting.
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        step_count: Current training step count
        k: Key for performance metric ("TPS" or "latency")
        **kwargs: Additional keyword arguments
        
    Returns:
        Combined multi-objective reward
    """
    # Base reward from improvement over previous state
    delta_pre = np.sign(obs - performance_history[k][-1]) * np.log1p(abs(obs - performance_history[k][-1]))
    reward_pre = delta_pre / (abs(delta_pre) + 1)
    
    # Reward from improvement over best historical performance
    if k == 'latency':
        mean_top_value = statistics.mean(heapq.nsmallest(3, performance_history[k]))
    elif k == 'TPS':
        mean_top_value = statistics.mean(heapq.nlargest(3, performance_history[k]))
    else:
        assert False, f"Invalid name {k}"
        
    delta_best = np.sign(obs - mean_top_value) * np.log1p(abs(obs - mean_top_value))
    reward_best = delta_best / (abs(delta_best) + 1)
    
    # Reward for stability (less variance in recent performance)
    recent_perf = performance_history[k][-10:] if len(performance_history[k]) > 10 else performance_history[k]
    variance = np.var(recent_perf) if len(recent_perf) > 1 else 0.0
    norm_variance = min(1.0, variance / (np.mean(recent_perf) + 1e-8))
    stability_reward = 0.2 * (1.0 - norm_variance)  # Higher when more stable
    
    # Combine with weights that change over time
    progress = min(1.0, step_count / 5000)
    w1 = 0.8 - (0.3 * progress)  # Weight for immediate improvement (decreases over time)
    w2 = 0.2 + (0.3 * progress)  # Weight for best historical (increases over time)
    
    return (w1 * reward_pre) + (w2 * reward_best) + stability_reward

OPTIMAL_CONSENSUS = {
    # Format: {is_bft, condition_index}: [primary_algo, secondary_algo]
    (False, 0): ["paxos", "vpaxos"],       # Small-Scale, Read-Heavy
    (False, 1): ["epaxos", "vpaxos"],      # Medium-Scale, Balanced
    (False, 2): ["wpaxos", "epaxos"],      # Large-Scale, Write-Heavy
    (True, 3): ["pbft", "mirbft"],         # Security-Critical
    (True, 4): ["chainedhotstuff", "fasthotstuff"]  # High-Throughput, Security-Critical
}

def calculate_bft_penalty(is_bft_mode, algorithm, current_latency):
    """
    Calculate penalty for using inappropriate security level.
    Returns negative value (penalty) if security mismatches requirements.
    """
    # BFT algorithms
    bft_algorithms = ["pbft", "mirbft", "chainedhotstuff", "fasthotstuff"]
    # CFT algorithms
    cft_algorithms = ["paxos", "epaxos", "vpaxos", "wpaxos"]
    
    if is_bft_mode and algorithm in cft_algorithms:
        # Severe penalty for using CFT algorithm in BFT environment
        # (security breach potential)
        return -2.0
    elif not is_bft_mode and algorithm in bft_algorithms:
        # Moderate penalty for using BFT algorithm in CFT environment
        # (unnecessary overhead)
        penalty = -0.5 * (current_latency / 100)  # Scale by latency
        return max(-1.0, penalty)  # Cap at -1.0
    
    # No penalty - appropriate security level
    return 0.0

def calculate_workload_alignment(algorithm, writeratio, nodesize):
    """
    Calculate how well the algorithm aligns with the workload characteristics.
    Returns a score between -0.5 and 0.5.
    """
    score = 0.0
    
    # Read-heavy workload optimizations
    if writeratio <= 0.3:
        if algorithm in ["paxos", "vpaxos"]:
            score += 0.3
        elif algorithm in ["epaxos"]:
            score += 0.1
    
    # Balanced workload
    elif writeratio <= 0.6:
        if algorithm in ["epaxos", "vpaxos"]:
            score += 0.3
        elif algorithm in ["paxos", "wpaxos"]:
            score += 0.1
    
    # Write-heavy workload
    else:
        if algorithm in ["wpaxos", "epaxos"]:
            score += 0.3
        elif algorithm in ["paxos"]:
            score -= 0.1
    
    # Scale adjustments
    if nodesize >= 16:
        if algorithm in ["wpaxos", "epaxos", "chainedhotstuff"]:
            score += 0.2
        elif algorithm in ["paxos", "pbft"]:
            score -= 0.2
    
    return max(-0.5, min(0.5, score))  # Clamp between -0.5 and 0.5

def adaptive_consensus_reward(
    obs, 
    performance_history, 
    k="TPS", 
    is_bft_mode=False,
    discount=0.8, 
    max_top=3,
    step_count=0,
    total_steps=5000,
    **kwargs
):
    """
    Reward function optimized for consensus protocol selection.
    
    Args:
        obs: Current observation (performance metric)
        performance_history: History of performance metrics
        k: Key for performance metric ("TPS" or "latency")
        is_bft_mode: Flag indicating if the system is in BFT mode
        discount: Discount factor for previous vs best performance
        max_top: Number of top performances to consider
        step_count: Current training step count
        **kwargs: Additional keyword arguments
        
    Returns:
        Calculated reward with consensus-specific adjustments
    """
    if total_steps <= 0:
        total_steps = 1000
    # Calculate condition set index based on step count
    condition_duration = max(1, total_steps // 5)  # 5 conditions in total
    # condition_duration = 10
    logger.info(f"######## condition_duration: {total_steps}, {condition_duration}")

    condition_index = (step_count // condition_duration) % 5
    # condition_index = min(4, (step_count // condition_duration))
    
    # Base reward calculation using best_and_pre logic
    if k == 'latency':
        mean_top_value = statistics.mean(heapq.nsmallest(max_top, performance_history[k])) if performance_history[k] else obs
    elif k == 'TPS':
        mean_top_value = statistics.mean(heapq.nlargest(max_top, performance_history[k])) if performance_history[k] else obs
    else:
        assert False, f"Invalid name {k}"
    
    delta_best = np.sign(obs - mean_top_value) * np.log1p(abs(obs - mean_top_value))
    
    # For the previous performance comparison, use more recent history
    prev_value = performance_history[k][-1] if performance_history[k] else obs
    delta_pre = np.sign(obs - prev_value) * np.log1p(abs(obs - prev_value))
    
    reward_best = delta_best / (abs(delta_best) + 1)
    reward_pre = delta_pre / (abs(delta_pre) + 1)
    
    # Base reward
    base_reward = discount * reward_pre + (1 - discount) * reward_best
    
    # Try to get the current algorithm from the action file
    try:
        # Assuming the action file is in the expected location
        action_file = os.path.join(kwargs.get('project_dir', '.'), 'config_RL.json')
        if os.path.exists(action_file):
            with open(action_file, 'r') as f:
                config = json.load(f)
                current_algorithm = config.get('algorithm', None)
                writeratio = config.get('writeratio', 0.5)
                nodesize = config.get('nodesize', 4)
        else:
            logger.warning(f"Action file not found: {action_file}")
            current_algorithm = None
            writeratio = 0.5
            nodesize = 4
    except Exception as e:
        logger.error(f"Error reading action file: {e}")
        current_algorithm = None
        writeratio = 0.5
        nodesize = 4
    
    # Apply consensus-specific adjustments if we have the algorithm info
    if current_algorithm:
        # Security-level penalty (BFT vs CFT)
        security_penalty = calculate_bft_penalty(is_bft_mode, current_algorithm, obs if k == 'latency' else 100)
        
        # Workload alignment bonus/penalty
        workload_alignment = calculate_workload_alignment(current_algorithm, writeratio, nodesize)
        
        # Check if the algorithm matches the expected optimal for this condition
        optimal_key = (is_bft_mode, condition_index)
        optimal_boost = 0.0
        
        if optimal_key in OPTIMAL_CONSENSUS:
            if current_algorithm == OPTIMAL_CONSENSUS[optimal_key][0]:
                # Primary optimal algorithm
                optimal_boost = 0.3
            elif current_algorithm == OPTIMAL_CONSENSUS[optimal_key][1]:
                # Secondary optimal algorithm
                optimal_boost = 0.15
        
        # Add adjustments to base reward
        adjusted_reward = base_reward + security_penalty + workload_alignment + optimal_boost
        
        # Log the reward components
        logger.info(f"Reward components - Base: {base_reward:.3f}, Security: {security_penalty:.3f}, " +
                    f"Workload: {workload_alignment:.3f}, Optimal: {optimal_boost:.3f}, " +
                    f"Final: {adjusted_reward:.3f}")
        
        # Ensure reward is not negative
        return max(0, adjusted_reward)
    
    # If we couldn't get algorithm info, just return the base reward
    return max(0, base_reward)

def result_logging_reward(
    obs, 
    performance_history, 
    k="TPS", 
    is_bft_mode=False,
    step_count=0,
    **kwargs
):
    """
    Special reward function that logs detailed results for analysis.
    Uses the adaptive_consensus_reward underneath but adds extensive logging.
    """
    # Calculate the actual reward using the adaptive function
    reward = adaptive_consensus_reward(
        obs, performance_history, k, is_bft_mode, step_count=step_count, **kwargs
    )
    
    # Try to extract the current environment and algorithm information
    try:
        # Assuming the action file is in the expected location
        action_file = os.path.join(kwargs.get('project_dir', '.'), 'config_RL.json')
        if os.path.exists(action_file):
            with open(action_file, 'r') as f:
                config = json.load(f)
                
            # Extract key parameters for logging
            algorithm = config.get('algorithm', 'unknown')
            nodesize = config.get('nodesize', 0)
            clients = config.get('clients', 0)
            writeratio = config.get('writeratio', 0)
            concurrency = config.get('concurrency', 0)
            ratelimit = config.get('ratelimit', 0)
            batchsize = config.get('batchsize', 0)
            payloadsize = config.get('payloadsize', 0)
            
            # Get the performance metrics
            tps = obs if k == "TPS" else kwargs.get('tps_value', 0)
            latency = obs if k == "latency" else kwargs.get('latency_value', 0)
            
            # Calculate condition set index based on step count
            condition_duration = kwargs.get('total_steps', 5000) // 5  # 5 conditions
            condition_index = min(4, (step_count // condition_duration))
            
            # Log to a CSV file for later analysis
            results_file = os.path.join(kwargs.get('saved_folder', '.'), 'experiment_results.csv')
            file_exists = os.path.exists(results_file)
            
            with open(results_file, 'a') as f:
                if not file_exists:
                    # Write header if file doesn't exist
                    f.write("step,condition,algorithm,bft,nodesize,clients,writeratio,concurrency,ratelimit,batchsize,payloadsize,tps,latency,reward\n")
                
                # Write the data row
                f.write(f"{step_count},{condition_index},{algorithm},{is_bft_mode},{nodesize},{clients},{writeratio},{concurrency},{ratelimit},{batchsize},{payloadsize},{tps},{latency},{reward}\n")
                
            logger.info(f"Logged result - Step: {step_count}, Condition: {condition_index}, Algorithm: {algorithm}, Reward: {reward:.3f}")
    
    except Exception as e:
        logger.error(f"Error logging experiment results: {e}")
    
    return reward