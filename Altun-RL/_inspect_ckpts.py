"""Inspect MADDPG checkpoints and report state_dims / action_dims.

Usage: python _inspect_ckpts.py  (runs on a machine with torch available)
"""
import torch, os, json, sys

CANDIDATES = [
    # (label, path)
    ("adapt_09-15", "resultsv4-rl/adapt/log_adaptive_consensus_reward_env_adaptive_consensus_mutation_202509141604/models/MADDPG_trained_agent_1000_final.pt"),
    ("lr_09-10b",  "resultsv4-rl/lr/log_best_and_pre_env_ratelimit_mutation_202509091647/models/MADDPG_trained_agent_1000_final.pt"),
    ("lr_09-10a",  "resultsv4-rl/lr/log_best_and_pre_env_ratelimit_mutation_202509091627/models/MADDPG_trained_agent_1000_final.pt"),
    ("disc_09-10", "resultsv4-rl/discount/log_best_and_pre_env_ratelimit_mutation_202509090605/models/MADDPG_trained_agent_1000_final.pt"),
    ("disc_09-09b","resultsv4-rl/discount/log_best_and_pre_env_ratelimit_mutation_202509082033/models/MADDPG_trained_agent_1000_final.pt"),
    ("disc_09-09a","resultsv4-rl/discount/log_best_and_pre_env_ratelimit_mutation_202509082031/models/MADDPG_trained_agent_1000_final.pt"),
    ("rl_0_05-01", "resultsv2/ratelimit/ratelimit_0_202504292209/models/MADDPG_trained_agent_1000_final.pt"),
    ("rl_20_04-30","resultsv2/ratelimit/ratelimit_20_202504292218/models/MADDPG_trained_agent_1000_final.pt"),
    ("sel_04-26",  "resultsv2/selection/selection_202504251739/models/MADDPG_trained_agent_2000_final.pt"),
    ("bat_04-28",  "resultsv2/batch/batchsize_202504271723/models/MADDPG_trained_agent_1000_final.pt"),
    ("ns_04-12",   "resultsv2/nodesize/nodesize_202504111002/models/MADDPG_trained_agent_1000_final.pt"),
    ("cc_04-11",   "resultsv2/concurrency/concurrency_202504091408/models/MADDPG_trained_agent_1000_final.pt"),
    ("w88_2024",   "/data-ssd/tianwen/experiments/random_change/log_only_best_and_pre_1_wo_ratio_env_mutation_mutate_env_tps_w88/models/MADDPG_trained_agent_500.pt"),
]

print(f"{'label':<14} {'state_dims':<15} {'action_dims':<12} log_exists")
print("=" * 70)
for label, rel in CANDIDATES:
    path = rel if rel.startswith("/") else os.path.join(os.path.dirname(__file__), rel)
    if not os.path.exists(path):
        print(f"{label:<14} MISSING        -            -    {path}")
        continue
    try:
        ck = torch.load(path, map_location="cpu", weights_only=False)
    except Exception as e:
        print(f"{label:<14} LOAD_ERR: {e}")
        continue
    sd = ck.get("state_dims")
    ad = ck.get("action_dims")
    log_dir = os.path.dirname(os.path.dirname(path))
    log_glob = [f for f in os.listdir(log_dir) if f.endswith(".log") or f.endswith(".json")]
    print(f"{label:<14} {str(sd):<15} {str(ad):<12} {','.join(log_glob[:2]) if log_glob else 'none'}")
