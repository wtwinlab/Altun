import os
import json
import sys

def parse_exp_log(file_name, ouptutfile):
    data = open(file_name).read()
    lines = data.splitlines()

    data_pairs = []
    with open(ouptutfile, "w") as f:
        f.write("alg,concurrency,replicas,clients,batchsize,payloadsize,leaderrotation,ratelimit,latency,TPS\n")
    data_lines = []
    for i in range(len(lines) - 1, -1, -1):
        if lines[i].strip().startswith("{"):
            parameters = lines[i - 1].strip()
            values = parameters.split("-")
            # print(values)
            if len(values[-1]) == 0:
                values[-1] = "0"
            
            sv = [ ]
            if len(values) == 9:
                values[-3] = values[-3] + "-" + values[-2] 
                del values[-2] 
            performance = json.loads(lines[i].strip())
            tps = str(performance["TPS"])
            latency = str(performance["latency"])
            data_pairs.insert(0, (parameters, performance))
            data_lines.append(",".join(values+[tps, latency])+"\n")
        else:
            continue
    # # Print the parsed data pairs
    # for pair in data_pairs:
    #     print(pair)
    # print(len(data_pairs))
    with open(ouptutfile, "a") as f:
        f.writelines(data_lines)
    

if __name__ == "__main__":
    parse_exp_log(sys.argv[1], sys.argv[2])