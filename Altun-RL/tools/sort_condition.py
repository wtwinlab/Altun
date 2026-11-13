#!/usr/bin/env python3
import pandas as pd
import argparse

def sort_csv_data(input_file, output_file, primary_sort_column, ascending=True):
    """
    Sorts a CSV file by a specified primary column and then by algorithm.
    
    Args:
        input_file (str): Path to the input CSV file
        output_file (str): Path to save the sorted CSV file
        primary_sort_column (str): Name of column to use for primary sorting
        ascending (bool): Sort direction (True for ascending, False for descending)
    """
    try:
        # Read the CSV file
        print(f"Reading data from: {input_file}")
        df = pd.read_csv(input_file)
        
        # Check if primary sort column exists
        if primary_sort_column not in df.columns:
            print(f"Error: Column '{primary_sort_column}' not found in the CSV file.")
            print(f"Available columns: {', '.join(df.columns)}")
            return False
        
        # Check if 'consensus' column exists (for secondary sorting)
        secondary_sort_column = 'consensus'
        if secondary_sort_column not in df.columns:
            print(f"Warning: Column '{secondary_sort_column}' not found. Sorting only by '{primary_sort_column}'.")
            # Sort by primary column only
            sorted_df = df.sort_values(by=primary_sort_column, ascending=ascending)
        else:
            # Sort by primary column, then by algorithm
            sorted_df = df.sort_values(by=[primary_sort_column, secondary_sort_column], 
                                      ascending=[ascending, True])
        
        # Save the sorted data
        sorted_df.to_csv(output_file, index=False)
        print(f"Successfully saved sorted data to {output_file}")
        
        # Print sorting statistics
        primary_column_values = sorted_df[primary_sort_column].unique()
        print(f"\nSorting Statistics:")
        print(f"- Sorted primarily by: {primary_sort_column}")
        print(f"- Number of unique {primary_sort_column} values: {len(primary_column_values)}")
        
        if secondary_sort_column in df.columns:
            algorithm_values = sorted_df[secondary_sort_column].unique()
            print(f"- Sorted secondarily by: {secondary_sort_column}")
            print(f"- Number of unique {secondary_sort_column} values: {len(algorithm_values)}")
            print(f"- Algorithms found: {', '.join(map(str, algorithm_values))}")
        
        return True
        
    except Exception as e:
        print(f"Error processing file: {e}")
        return False

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Sort CSV data by parameter and algorithm")
    parser.add_argument("--input", required=True, help="Path to input CSV file")
    parser.add_argument("--output", default="sorted_results.csv", help="Path to save sorted output CSV")
    parser.add_argument("--column", required=True, help="Primary column to sort by")
    parser.add_argument("--descending", action="store_true", help="Sort in descending order")
    
    args = parser.parse_args()
    # sort_csv_data(args.input, args.output, args.column, not args.descending)
    sort_csv_data(args.input, args.output, args.column, args.descending)