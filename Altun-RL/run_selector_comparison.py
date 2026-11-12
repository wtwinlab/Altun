"""
Experiment 1: Selector Justification (fills Table 5).

Runs baseline selectors (Thompson, UCB1, Greedy, BestFixed) against the
configured mutation function. DDPG column comes from the training log
via tools/build_exp1_table.py, not from a live run (unless --include_ddpg).
Results written to <saved_folder>/selector_comparison.csv.
"""
import os, json, csv, time, argparse, copy
import numpy as np
import torch
from maddpg_env import VirtualEnv_MADDPG
from baseline_selectors import ThompsonSamplingSelector, UCB1Selector, GreedySelector, BestFixedSelector, FixedProtocolSelector
from agilerl.algorithms.maddpg import MADDPG


def load_ddpg_agent(env, args):
    """Load MADDPG agent from checkpoint for inference.

    Construction mirrors online_maddpg.py:197 so the checkpoint loads
    into a network with the same shape as the one it was trained with.
    """
    try:
        state_dim = [env.observation_space(a).n for a in env.agents]
        one_hot = True
    except Exception:
        state_dim = [env.observation_space(a).shape for a in env.agents]
        one_hot = False
    action_dim = [env.action_space(a).n for a in env.agents]
    agent = MADDPG(
        state_dims=state_dim,
        action_dims=action_dim,
        one_hot=one_hot,
        n_agents=env.num_agents,
        agent_ids=env.agents,
        max_action=None,
        min_action=None,
        vect_noise_dim=1,
        discrete_actions=True,
        device=args.device,
    )
    if args.checkpoint and os.path.isfile(args.checkpoint):
        agent.load_checkpoint(args.checkpoint)
        print(f"[DDPG] Loaded checkpoint: {args.checkpoint}")
    else:
        print("[DDPG] No checkpoint found; using random policy.")
    return agent


def alg_name_from_action(action_dict, env):
    """Read the selected algorithm name from config_RL.json after _take_action."""
    try:
        return json.load(open(env.config['action_file']))['algorithm']
    except Exception:
        return 'unknown'


def run_one_selector(selector_name, args):
    env = VirtualEnv_MADDPG(args.config_file, 0, args.reward, args.record_file, args.saved_folder, args)
    obs, info = env.reset()

    cfg      = json.load(open(args.config_file))
    bft_algs = list(cfg['action_space']['discrete']['bft_alg'])
    cft_algs = list(cfg['action_space']['discrete']['cft_alg'])
    # Unified pool the baselines pick from. cft_algs is already the superset
    # (it contains every BFT protocol as well), but build a dedup union in
    # case config_default.json is edited later.
    all_algs = list(dict.fromkeys(cft_algs + bft_algs))
    # Optional per-run exclusion — used by Exp 1 v2 to skip pbft/mirbft
    # whose mir/deployment pipeline is broken on the current cluster
    # (see exp_logs/exp1_log_based_analysis.md §7 gap 3).
    if getattr(args, 'exclude_protocols', None):
        excluded = [p.strip() for p in args.exclude_protocols.split(',') if p.strip()]
        all_algs = [a for a in all_algs if a not in excluded]
        print(f"[run_selector_comparison] excluded protocols: {excluded} "
              f"→ pool={all_algs}")

    if selector_name == 'thompson':
        sel = ThompsonSamplingSelector(all_algs)
        ddpg = None
    elif selector_name == 'ucb1':
        sel = UCB1Selector(all_algs)
        ddpg = None
    elif selector_name == 'greedy':
        sel = GreedySelector(all_algs)
        ddpg = None
    elif selector_name == 'best_fixed':
        sel = BestFixedSelector(all_algs)
        ddpg = None
    elif selector_name.endswith('_fixed'):
        proto = selector_name.replace('_fixed', '')
        sel = FixedProtocolSelector(all_algs, proto)
        ddpg = None
    else:  # ddpg
        sel  = None
        ddpg = load_ddpg_agent(env, args)

    records, prev_alg = [], None

    for step in range(args.max_steps):
        # env_bft is logged for post-hoc analysis only — the baselines are
        # NOT given this flag. See baseline_selectors.py docstring.
        workload_is_bft = env.mutation_env.get('env_bft', False)

        t_dec_start = time.time()
        if sel is not None:
            intended_alg = sel.select()
            # Route the intended pick through both action heads: the env's
            # family gate (_take_action, maddpg_env.py:542) uses bft_alg on
            # BFT blocks and cft_alg on CFT blocks. When the intended alg is
            # not in the head the env uses, the fallback index 0 is applied.
            bft_idx = bft_algs.index(intended_alg) if intended_alg in bft_algs else 0
            cft_idx = cft_algs.index(intended_alg) if intended_alg in cft_algs else 0
            action_dict = {
                'bft_alg': np.array([[bft_idx]]),
                'cft_alg': np.array([[cft_idx]]),
            }
        else:
            intended_alg = None
            _, discrete_action = ddpg.get_action(obs, False, info)
            action_dict = discrete_action
        t_dec_ms = (time.time() - t_dec_start) * 1000

        obs, reward_dict, _, _, info = env.step(action_dict)

        applied_alg = alg_name_from_action(action_dict, env)
        tps         = env.init_info.get('TPS', 0)
        latency     = env.init_info.get('latency', 0)
        reward      = float(list(reward_dict.values())[0]) if reward_dict else 0.0

        # Baseline updates its history against the protocol it *intended* to
        # pick, using the TPS it actually observed. This is the honest blind
        # feedback a selector without oracle would see.
        if sel is not None:
            sel.update(intended_alg, tps, reward)

        records.append({
            'step':         step,
            'selector':     selector_name,
            'intended_alg': intended_alg if intended_alg is not None else applied_alg,
            'algorithm':    applied_alg,
            'is_bft':       int(workload_is_bft),
            'TPS':          round(tps, 2),
            'latency':      round(latency, 2),
            'reward':       round(reward, 4),
            't_decide_ms':  round(t_dec_ms, 3),
            'switched':     int(applied_alg != prev_alg and prev_alg is not None),
            'block':        step // 8,
        })
        prev_alg = applied_alg

    return records


def main():
    parser = argparse.ArgumentParser(description='Selector Justification Experiment')
    parser.add_argument('--config_file',   required=True)
    parser.add_argument('--project_dir',   required=True)
    parser.add_argument('--saved_folder',  required=True)
    parser.add_argument('--max_steps',     type=int,   default=96,
                        help='12 blocks × 8 steps per block')
    parser.add_argument('--checkpoint',    type=str,   default=None)
    parser.add_argument('--reward',        default='TPS', choices=['TPS', 'latency'])
    parser.add_argument('--reward_fn',     default='best_and_pre')
    parser.add_argument('--mutate_env_fn', default='block_schedule_mutation')
    parser.add_argument('--reward_discount', type=float, default=0.8)
    parser.add_argument('--reward_top_num',  type=int,   default=3)
    parser.add_argument('--cooldown_steps',  type=int,   default=0)
    parser.add_argument('--switch_margin',   type=float, default=0.0)
    parser.add_argument('--device', default='cuda' if torch.cuda.is_available() else 'cpu')
    parser.add_argument('--include_ddpg', action='store_true',
                        help='Also run DDPG live (legacy mode). By default, skipped: the '
                             'DDPG column comes from the training log via '
                             'tools/build_exp1_table.py.')
    parser.add_argument('--exclude_protocols', type=str, default='',
                        help='Comma-separated list of protocol names to remove from the '
                             'baseline action pool (e.g. "pbft,mirbft" for v2 where the '
                             'mir deployment pipeline is broken on current cluster).')
    parser.add_argument('--selectors', type=str, default='',
                        help='Comma-separated list of selectors to run (e.g. "thompson,ucb1"). '
                             'Supports: thompson, ucb1, greedy, best_fixed, <proto>_fixed '
                             '(e.g. paxos_fixed, vpaxos_fixed). Overrides default list.')
    args = parser.parse_args()

    os.makedirs(args.saved_folder, exist_ok=True)

    if args.selectors:
        selector_list = [s.strip() for s in args.selectors.split(',') if s.strip()]
    elif args.include_ddpg:
        selector_list = ['thompson', 'ucb1', 'greedy', 'best_fixed', 'ddpg']
    else:
        selector_list = ['thompson', 'ucb1', 'greedy', 'best_fixed']

    all_records = []
    for sel_name in selector_list:
        print(f'\n{"="*50}\nSelector: {sel_name}\n{"="*50}')
        sub_args = copy.copy(args)
        sub_args.record_file = os.path.join(args.saved_folder, f'{sel_name}_log.csv')
        records = run_one_selector(sel_name, sub_args)
        all_records.extend(records)
        print(f'  mean TPS={np.mean([r["TPS"] for r in records]):.0f}  '
              f'switches={sum(r["switched"] for r in records)}')

    out = os.path.join(args.saved_folder, 'selector_comparison.csv')
    with open(out, 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=all_records[0].keys())
        writer.writeheader()
        writer.writerows(all_records)
    print(f'\nAll done. Results: {out}')


if __name__ == '__main__':
    main()
