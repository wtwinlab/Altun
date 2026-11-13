#!/usr/bin/env python3
import pandas as pd
import argparse
import os

def extract_representative_records(input_file, output_file, param_name):
    """
    Extracts representative records with non-zero TPS for each unique combination
    of consensus algorithm and parameter value.
    
    Args:
        input_file (str): Path to the input CSV file
        output_file (str): Path to save the filtered CSV file
        param_name (str): Name of the parameter column to use
    """
    try:
        # Read the CSV file
        print(f"Reading data from: {input_file}")
        df = pd.read_csv(input_file)
        
        # Check if required columns exist
        required_columns = ['id', 'TPS', 'latency', 'consensus', param_name]
        missing_columns = [col for col in required_columns if col not in df.columns]
        
        if missing_columns:
            print(f"Error: The following required columns are missing: {', '.join(missing_columns)}")
            print(f"Available columns: {', '.join(df.columns)}")
            return False
        
        # Filter out zero TPS records
        filtered_df = df[df['TPS'] > 0].copy()
        
        if filtered_df.empty:
            print("Error: No records with non-zero TPS found in the input file.")
            return False
        
        print(f"Found {len(filtered_df)} records with non-zero TPS.")
        
        # Get unique combinations of consensus and parameter
        unique_combinations = filtered_df.groupby(['consensus', param_name]).size().reset_index().rename(columns={0: 'count'})
        print(f"Found {len(unique_combinations)} unique combinations of consensus algorithm and {param_name}.")
        
        # Select one record for each combination (the one with the highest TPS)
        representative_records = []
        
        for _, row in unique_combinations.iterrows():
            consensus_value = row['consensus']
            param_value = row[param_name]
            
            matching_records = filtered_df[
                (filtered_df['consensus'] == consensus_value) & 
                (filtered_df[param_name] == param_value)
            ]
            
            # Get the record with the highest TPS
            best_record = matching_records.loc[matching_records['TPS'].idxmax()]
            representative_records.append(best_record)
        
        # Create a DataFrame from the representative records
        result_df = pd.DataFrame(representative_records)
        
        # Select only the required columns
        result_df = result_df[required_columns]
        
        # Save the result
        result_df.to_csv(output_file, index=False)
        print(f"Successfully saved {len(result_df)} representative records to {output_file}")
        
        # Print statistics about the result
        consensus_count = result_df['consensus'].nunique()
        param_values_count = result_df[param_name].nunique()
        
        print(f"\nOutput Statistics:")
        print(f"- Total representative records: {len(result_df)}")
        print(f"- Number of unique consensus algorithms: {consensus_count}")
        print(f"- Number of unique {param_name} values: {param_values_count}")
        print(f"- Algorithms included: {', '.join(result_df['consensus'].unique())}")
        print(f"- {param_name} values included: {', '.join(map(str, sorted(result_df[param_name].unique())))}")
        
        return True
        
    except Exception as e:
        print(f"Error processing file: {e}")
        return False

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Extract representative records for each consensus/parameter combination")
    parser.add_argument("--input", required=True, help="Path to input CSV file")
    parser.add_argument("--output", default="representative_results.csv", help="Path to save output CSV")
    parser.add_argument("--param", required=True, help="Parameter column name to use for filtering")
    
    args = parser.parse_args()
    
    extract_representative_records(args.input, args.output, args.param)