import os
import json
import re
import json
import os
import time
from datetime import datetime
import subprocess
from loguru import logger
import threading

# Function to read the file content
def read_file_with_retry(file_path, retries=5, delay=0.1):
    for attempt in range(retries):
        try:
            with open(file_path, 'r') as file:
                content = json.load(file)
            return content
        except (IOError, json.JSONDecodeError):
            time.sleep(delay)
    raise Exception(f"Failed to read file {file_path} after {retries} attempts")


def check_if_run_success(system_performance_file):
    if not os.path.isfile(system_performance_file):
        return False,[]
    data, raw_data = read_tps_and_latency_values(system_performance_file)
    if data['TPS'] is None or data['latency'] is None:
        return False, data, {}
    return True, data, raw_data

def parse_observation_file(system_performance_file, system_default_configuraiton_file, obs_space_name,\
                           env_mutation_file=None, applied_config_file=None, action_space_name=None, environments=[]):
    #print(system_performance_file)
    system_performance = { }
    if system_performance_file is not None:
        system_performance, _ = read_tps_and_latency_values(system_performance_file)

    if len(system_performance) != 0:
        if system_performance["TPS"] is None or system_performance['latency'] is None:
            logger.warning("TPS and latency from Current performance file are None. Please check {system_performance_file}")


    # read the default env config
    default_system_config = read_file_with_retry(system_default_configuraiton_file)
    bench_marks = default_system_config['benchmark']

    obs_dict = {}
    for n in obs_space_name:
        if n in bench_marks:
            obs_dict[n] = bench_marks[n]
        if n in default_system_config:
            obs_dict[n] = default_system_config[n]
    current_RL_config = {}
    if applied_config_file is not None and os.path.isfile(applied_config_file):
        current_RL_config = read_file_with_retry(applied_config_file)
        for n in action_space_name:
            if n in current_RL_config['benchmark']:
                obs_dict[n] = bench_marks[n]
            if n in current_RL_config:
                obs_dict[n] = current_RL_config[n]
    env_mutation = {}
    logger.info(f"env_mutation_file {env_mutation_file}")
    if env_mutation_file is not None and os.path.isfile(env_mutation_file):
        env_mutation_org = read_file_with_retry(env_mutation_file)
        for e in environments:
            env_mutation[f'env_{e}'] = env_mutation_org[f'env_{e}']
    else:
        for e in environments:
            if e in bench_marks:
                env_mutation[f'env_{e}'] = bench_marks[e]
            else:
                env_mutation[f'env_{e}'] = default_system_config[e]
   
    for e in env_mutation:
        if e == "env_Distribution":
            logger.info(f"env_Distribution {type(env_mutation[e])} {env_mutation[e][0]}")
            if isinstance(env_mutation[e], str):
                obs_dict["env_Distribution"] = env_mutation[e]
            else: 
                obs_dict["env_Distribution"] = env_mutation[e][0]
                if  env_mutation[e][1] is not None :
                    obs_dict["Conflicts"] = env_mutation[e][1]
            continue
        obs_dict[e] = env_mutation[e]
    logger.info(f"obs_dict")
    logger.info(obs_dict)
    # print(obs_dict)
    # print(system_performance)
    obs_dict.update(system_performance)
    return obs_dict

def read_tps_and_latency_values(benchmark_log_file):
    logger.info(f"read_tps_and_latency_values ==== {benchmark_log_file}")
    data = open(benchmark_log_file).read()

    tps_values = []
    latency_values = []

    current_key = None
    import re

    parts = data.split(",")

    for part in parts:
            part = part.strip()
            if not part:
                continue
            if '"TPS":' in part:
                current_key = 'TPS'
                # Extract numbers after the key
                numbers = re.findall(r"[-+]?\d*\.\d+|\d+", part)
                for num in numbers:
                    tps_values.append(float(num))
            elif '"latency":' in part:
                current_key = 'latency'
                numbers = re.findall(r"[-+]?\d*\.\d+|\d+", part)
                for num in numbers:
                    latency_values.append(float(num))
            else:
                assert False, f"{data}"

    # Retrieve the last values
    last_tps = tps_values[-1] if tps_values else None
    last_latency = latency_values[-1] if latency_values else None
    logger.info(f"data values: {data}")
    logger.info(f"TPS values: {tps_values}")
    logger.info(f"Latency values: {latency_values}")
    logger.info(f"Last TPS value: {last_tps}")
    logger.info(f"Last latency value: {last_latency}")
    return {"TPS":last_tps, "latency":last_latency}, data


def update_and_run_config(config_path, new_config_path, action_updated_config, history_dir, project_dir, action_values,mutation_env=None, \
                          threshold_time=300, max_retries=3, run_system=True):
    """
    Update the configuration file and run the corresponding bash script.
    If the subprocess runs for more than the specified threshold time, it will be terminated and retried.
    
    Args:
        config_path (str): Path to the original config file.
        new_config_path (str): Path to save the updated config file.
        update_dict (dict): Dictionary containing updates to be applied to the config file.
        history_dir (str): Directory to store the backup config files.
        threshold_time (int): Maximum time in seconds to allow the subprocess to run.
        max_retries (int): Number of times to retry running the subprocess if it exceeds threshold time.
    """
    # Load the current configuration
    with open(config_path, 'r') as file:
        config = json.load(file)
    
    # assert len(config["bft_config"])!=0, logger.info(f"{config_path}, {new_config_path}, {config["bft_config"]} {action_updated_config}")
    # logger.info(f"Updated BFT config {config["bft_config"]}")
    
    # Create the history directory if it doesn't exist
    if not os.path.exists(history_dir):
        os.makedirs(history_dir)
    
    logger.info(f"bft in previous config: {config["bft"]}")
    # # Create a backup of the current config with a timestamp
    # timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    # backup_filename = f"config_backup_{timestamp}.json"
    # backup_path = os.path.join(history_dir, backup_filename)
    
    # with open(backup_path, 'w') as backup_file:
    #     json.dump(config, backup_file, indent=4)
    
    # Helper function to update nested dictionaries
    for name in action_updated_config:
        if name in config:
            config[name] = action_updated_config[name]
        elif name in config['benchmark']:
            config['benchmark'][name] = action_updated_config[name]
        else:
            logger.error(f"{name} in action_updated_config is not updated in the config. Please check!")
            assert False
    
    # Update the config with the provided update dictionary
    updated_config = config
    
    # apply new env ["nconsensus","nREPLICAS", "nCLIENTS","nconcurrency", "nratelimit", "nbatchsize", "npayloadsize", "nleaderrotation"]
    for e in mutation_env:
        name = e.replace('env_', '')
        if name not in  ["nconsensus","nREPLICAS", "nCLIENTS","nconcurrency", "nratelimit", "nbatchsize", "npayloadsize", "nleaderrotation"]:
            if name == "Distribution":
                if not isinstance( mutation_env[e], str):
                    (dname, dvalue) = mutation_env[e]
                    updated_config['benchmark'][name] = dname
                    if dvalue is not None :
                        updated_config['benchmark']["Conflicts"] = dvalue 
                else:
                    updated_config['benchmark'][name] = mutation_env[e]
                continue
            if name in updated_config['benchmark']:
                updated_config['benchmark'][name] = mutation_env[e]
            elif name in updated_config:
                updated_config[name] = mutation_env[e]
            else:
                logger.error(f"{name} in mutation_env is not updated in the config. Please check!")
                assert False
        else:
            updated_config["bft_config"][name] =  mutation_env[e]

    # logger.info(f"Updated BFT config {updated_config["bft_config"]}")
    # check env
    if updated_config['bft']:
        if updated_config['algorithm'] not in  action_values['bft_alg']:
            assert False, f"Bft has no algorihtm { updated_config['algorithm']}"
        if 'bft_pol' in action_values:
            if updated_config['policy'] not in action_values['bft_pol']:
                assert False, f"Wrong BFT Policy {updated_config['policy']}"
    else:
        if updated_config['algorithm'] not in  action_values['cft_alg']:
            assert False, f"CFT has no algorihtm { updated_config['algorithm']}"
        if "cft_pol" in action_values:
            if updated_config['policy'] not in action_values['cft_pol']:
                assert False, f"Wrong CFT Policy {updated_config['policy']}"
    
    is_BFT_should_run_flag = None
    if updated_config['algorithm'] in action_values['bft_alg']:
        is_BFT_should_run_flag = True
    else:
        is_BFT_should_run_flag = False

    # Save the updated configuration back to the original file
    logger.info(updated_config)
    logger.info(f"{type(updated_config["benchmark"]["Distribution"])}")
    assert  isinstance(updated_config["benchmark"]["Distribution"], str)
    with open(new_config_path, 'w') as f:
        json.dump(updated_config, f, indent=4)
    
    
    with open("config_RL.json", "w") as f:
        json.dump(updated_config, f, indent=4)
    
    logger.info(f"New configs have been saved to {new_config_path}")
    logger.info(f'bft  -> {updated_config["bft"]}')
    # logger.info(f"Config updated and original saved to {backup_path}")

    # Function to run the bash script with a timeout
    def run_script():
        try:
            # Capture the output and error messages (if any)
            logger.info("Waiting for running config")
            # Run the command
            process = subprocess.Popen(['timeout', f'{threshold_time}s', 'bash', 'run_system.sh', project_dir, str(is_BFT_should_run_flag)], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            
            # # Set a timer to kill the process if it exceeds threshold_time
            # timer = threading.Timer(threshold_time, lambda: process.kill())
            # timer.start()

            start_time = time.time()

            while True:
                # line = process.stdout.readline()
                # if line:
                #     logger.info(f"{line.strip()}")
                if process.poll() is not None:
                    break
                if time.time() - start_time > threshold_time+5:
                    # Kill the process if it exceeds threshold_time
                    print(f"Process killed after exceeding threshold time of {threshold_time} seconds")
                    process.kill()
                    #process_kill = subprocess.Popen(['bash', 'kill.sh'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                    # subprocess.run(['bash', 'kill.sh'], shell=True, timeout=10)
                    print(f"Process killed after exceeding threshold time of {threshold_time} seconds")
                    time.sleep(5)  # Sleep briefly to avoid busy waiting
                    break
                time.sleep(0.1)  # Sleep briefly to avoid busy waiting

            montiro_output = process.stdout.read()
            logger.info(f"{montiro_output}")
            # Capture any error messages
            error_output = process.stderr.read()
            if error_output:
                logger.info(f"Error: {error_output}")
            
            # Wait for the process to complete
            process.wait()
            

            return process.returncode
        except subprocess.CalledProcessError as e:
            logger.error(f"Error occurred while running the script: {e}")
            logger.error(f"Script output: {e.stdout}")
            logger.error(f"Script errors: {e.stderr}")
            return -1
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            return -1

    # Retry logic
    retries = 0
    if run_system:
        while retries < max_retries:
            return_code = run_script()
            if return_code == 0:
                logger.info("Script completed successfully.")
                break
            else:
                retries += 1
                logger.warning(f"Script exceeded threshold time or failed. Retrying {retries}/{max_retries}...")
                time.sleep(5)  # Add delay before retrying
        else:
            logger.error("Max retries reached. Script did not complete successfully.")
    return updated_config


def apply_config(config_path, new_config_path, applied_config, history_dir, project_dir, \
                          threshold_time=300, max_retries=3, run_system=True):
    # Load the current configuration
    with open(config_path, 'r') as file:
        config = json.load(file)
    logger.info(f'bft in config {config["bft"]}')
    
    # Create the history directory if it doesn't exist
    if not os.path.exists(history_dir):
        os.makedirs(history_dir)
    
    # Create a backup of the current config with a timestamp
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    backup_filename = f"config_backup_{timestamp}.json"
    backup_path = os.path.join(history_dir, backup_filename)
    
    with open(backup_path, 'w') as backup_file:
        json.dump(config, backup_file, indent=4)
    

    # Save the updated configuration back to the original file
    with open(new_config_path, 'w') as f:
        json.dump(applied_config, f, indent=4)
    
    
    logger.info(f"New configs have been saved to {new_config_path}")
    logger.info(f"Config updated and original saved to {backup_path}")

    # Function to run the bash script with a timeout
    def run_script():
        try:
            # Capture the output and error messages (if any)
            logger.info("Waiting for running config")
            # Run the command
            process = subprocess.Popen(['timeout', f'{threshold_time}s', 'bash', 'run_system.sh', project_dir, str(applied_config['bft'])], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            
            start_time = time.time()

            while True:
                # line = process.stdout.readline()
                # if line:
                #     logger.info(f"{line.strip()}")
                if process.poll() is not None:
                    break
                if time.time() - start_time > threshold_time+5:
                    # Kill the process if it exceeds threshold_time
                    print(f"Process killed after exceeding threshold time of {threshold_time} seconds")
                    process.kill()
                    #process_kill = subprocess.Popen(['bash', 'kill.sh'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
                    # subprocess.run(['bash', 'kill.sh'], shell=True, timeout=10)
                    print(f"Process killed after exceeding threshold time of {threshold_time} seconds")
                    time.sleep(5)  # Sleep briefly to avoid busy waiting
                    break
                time.sleep(0.1)  # Sleep briefly to avoid busy waiting

            montiro_output = process.stdout.read()
            logger.info(f"{montiro_output}")
            # Capture any error messages
            error_output = process.stderr.read()
            if error_output:
                logger.info(f"Error: {error_output}")
            
            # Wait for the process to complete
            process.wait()
            

            return process.returncode
        except subprocess.CalledProcessError as e:
            logger.error(f"Error occurred while running the script: {e}")
            logger.error(f"Script output: {e.stdout}")
            logger.error(f"Script errors: {e.stderr}")
            return -1
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            return -1

    # Retry logic
    retries = 0
    if run_system:
        while retries < max_retries:
            return_code = run_script()
            if return_code == 0:
                logger.info("Script completed successfully.")
                break
            else:
                retries += 1
                logger.warning(f"Script exceeded threshold time or failed. Retrying {retries}/{max_retries}...")
                time.sleep(5)  # Add delay before retrying
        else:
            logger.error("Max retries reached. Script did not complete successfully.")



def update_config(config_path, new_config_path, action_updated_config, history_dir, project_dir, mutation_env, \
                          threshold_time=300, max_retries=3, run_system=True):
    """
    Update the configuration file and run the corresponding bash script.
    If the subprocess runs for more than the specified threshold time, it will be terminated and retried.
    
    Args:
        config_path (str): Path to the original config file.
        new_config_path (str): Path to save the updated config file.
        update_dict (dict): Dictionary containing updates to be applied to the config file.
        history_dir (str): Directory to store the backup config files.
        threshold_time (int): Maximum time in seconds to allow the subprocess to run.
        max_retries (int): Number of times to retry running the subprocess if it exceeds threshold time.
    """
    # Load the current configuration
    with open(config_path, 'r') as file:
        config = json.load(file)
    logger.info(f'bft in config {config["bft"]}')
    
    # Create the history directory if it doesn't exist
    if not os.path.exists(history_dir):
        os.makedirs(history_dir)
    
    # Create a backup of the current config with a timestamp
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    backup_filename = f"config_backup_{timestamp}.json"
    backup_path = os.path.join(history_dir, backup_filename)
    
    with open(backup_path, 'w') as backup_file:
        json.dump(config, backup_file, indent=4)
    
    # Helper function to update nested dictionaries
    for name in action_updated_config:
        if name in config:
            config[name] = action_updated_config[name]
        elif name in config['benchmark']:
            config['benchmark'][name] = action_updated_config[name]
        else:
            logger.error(f"{name} in action_updated_config is not updated in the config. Please check!")
            assert False
    
    # Update the config with the provided update dictionary
    updated_config = config
    
    # apply new env
    for e in mutation_env:
        name = e.replace('env_', '')
        if name in updated_config['benchmark']:
            updated_config['benchmark'][name] = mutation_env[e]
        elif name in updated_config:
            updated_config[name] = mutation_env[e]
        else:
            logger.error(f"{name} in mutation_env is not updated in the config. Please check!")
            assert False

    return updated_config


import os
import portalocker
import time

def wait_and_delete_file(file_path):
    isbft = False
    detection = False
    if os.path.exists(file_path):
        print(f"{file_path} exists, checking if it can be deleted...")
        while True:
            try:
                # Open the file to try and acquire an exclusive lock in non-blocking mode
                with open(file_path, 'r+') as f:
                    portalocker.lock(f, portalocker.LOCK_EX | portalocker.LOCK_NB)
                    # If lock is successful, proceed to delete the file
                    content = f.read().strip()
                    detection = True
                    if content == 'false':
                        isbft=False
                    if content == 'true':
                        isbft=True
                    break
            except portalocker.exceptions.LockException:
                # File is currently locked by another process, wait and retry
                print(f"{file_path} is in use by another process. Waiting...")
                time.sleep(1)

        # File is now available for deletion
        try:
            os.remove(file_path)
            print(f"{file_path} has been deleted successfully.")
            return detection, isbft
        except OSError as e:
            print(f"Error deleting {file_path}: {e}")
    else:
        print(f"{file_path} does not exist.")
    return None, None


def map_value_to_item(value, num_items):
    value = max(0, min(1, value))
    rw = 1.0/num_items
    for i in range(num_items):
        if value < (i+1) * rw:
            return i
    return num_items - 1
