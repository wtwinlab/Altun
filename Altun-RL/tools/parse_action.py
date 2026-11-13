#!/usr/bin/env python3
import os
import json
import csv
import pandas as pd

def parse_data(log_file, actions_dir, output_file, params_to_extract=None):
    """
    Parse data from log.csv and action files and save to output CSV.
    
    Args:
        log_file (str): Path to the log.csv file
        actions_dir (str): Path to the directory containing action_x.json files
        output_file (str): Path to save the output CSV file
        params_to_extract (list): List of parameters to extract from action files
    """
    print(f"Reading log data from: {log_file}")
    
    # Default parameters to extract if none specified
    if params_to_extract is None or len(params_to_extract) == 0:
        params_to_extract = ['nodesize']
        print("No parameters specified. Defaulting to extract 'nodesize'.")
    else:
        print(f"Parameters to extract: {', '.join(params_to_extract)}")
    
    # Check if log file exists
    if not os.path.exists(log_file):
        print(f"Error: Log file {log_file} does not exist.")
        return

    # Check if actions directory exists
    if not os.path.exists(actions_dir):
        print(f"Error: Actions directory {actions_dir} does not exist.")
        return
    
    # Read log.csv data
    try:
        log_data = pd.read_csv(log_file)
        print(f"Found {len(log_data)} records in log.csv")
    except Exception as e:
        print(f"Error reading log file: {e}")
        return
    
    # Prepare output data
    output_data = []
    
    # Process each row in log_data
    for index, row in log_data.iterrows():
        action_id = int(row['id'])
        action_file = os.path.join(actions_dir, f"action_{action_id}.json")
        
        # Check if action file exists
        if not os.path.exists(action_file):
            print(f"Warning: Action file {action_file} does not exist, skipping record with id {action_id}")
            continue
        
        try:
            # Read action file
            with open(action_file, 'r') as f:
                action_data = json.load(f)
            
            # Initialize record with log file data
            record = {
                'id': action_id,
                'TPS': row['TPS'],
                'latency': row['latency'],
                'consensus': row['consensus'],
            }
            
            # Add requested parameters from action file
            for param in params_to_extract:
                # record["consensus"] = action_data.get("algorithm")
                record[param] = action_data.get(param, None)
                
            output_data.append(record)
            
        except Exception as e:
            print(f"Error processing action file {action_file}: {e}")
    
    # Convert to DataFrame and save to CSV
    if output_data:
        df = pd.DataFrame(output_data)
        df.to_csv(output_file, index=False)
        print(f"Successfully saved {len(output_data)} records to {output_file}")
    else:
        print("No data to save.")

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Parse log.csv and action files")
    parser.add_argument("--log", default="log.csv", help="Path to log.csv file")
    parser.add_argument("--actions", default="history_action", help="Path to directory containing action_x.json files")
    parser.add_argument("--output", default="parsed_results.csv", help="Path to save output CSV file")
    parser.add_argument("--params", nargs='+', default=['nodesize'], 
                        help="Parameters to extract from action files (e.g. --params nodesize batchsize algorithm)")
    
    args = parser.parse_args()
    
    parse_data(args.log, args.actions, args.output, args.params)