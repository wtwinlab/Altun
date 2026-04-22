"""
Baseline selectors for Experiment 1 (Selector Justification, Table 5).

Fairness: these selectors must NOT see env_bft (the workload's BFT flag).
They pick from a single unified protocol pool using only observed TPS /
reward history. The driver routes whatever they pick through the env's
two-headed action interface; on BFT workload blocks, a CFT-family pick
is silently overridden to bft_alg[0] by the env's family gate.

Interface:
    select() -> str                    # returns protocol name
    update(alg: str, tps: float, reward: float)
"""
import numpy as np
from collections import defaultdict


class ThompsonSamplingSelector:
    """Beta(alpha, beta) per protocol over the unified pool."""
    def __init__(self, all_algs):
        self.algs  = list(all_algs)
        self.alpha = defaultdict(lambda: 1.0)
        self.beta  = defaultdict(lambda: 1.0)

    def select(self):
        return max(self.algs, key=lambda a: np.random.beta(self.alpha[a], self.beta[a]))

    def update(self, alg, tps, reward):
        if reward > 0:
            self.alpha[alg] += 1
        else:
            self.beta[alg] += 1


class GreedySelector:
    """Pick the protocol with the highest mean TPS in the last window_size steps."""
    def __init__(self, all_algs, window_size=5):
        self.algs    = list(all_algs)
        self.window  = window_size
        self.history = defaultdict(list)

    def select(self):
        return max(
            self.algs,
            key=lambda a: np.mean(self.history[a][-self.window:]) if self.history[a] else 0.0,
        )

    def update(self, alg, tps, reward):
        self.history[alg].append(tps)


class UCB1Selector:
    """Upper Confidence Bound (UCB1) bandit over the unified pool.

    UCB1 score = mean_TPS(arm) + c * sqrt(ln(t) / n_arm)

    Each arm is tried once during the first len(pool) steps (forced
    exploration), then UCB1 scores are used. c = sqrt(2) is the classic
    exploration coefficient from Auer et al. 2002.
    """
    def __init__(self, all_algs, c=None):
        self.algs    = list(all_algs)
        self.c       = c if c is not None else np.sqrt(2)
        self.counts  = defaultdict(int)     # n_arm
        self.totals  = defaultdict(float)   # cumulative TPS
        self.t       = 0                    # total pulls

    def select(self):
        # Forced exploration: each arm at least once
        for a in self.algs:
            if self.counts[a] == 0:
                return a
        # UCB1 score
        ln_t = np.log(self.t)
        return max(
            self.algs,
            key=lambda a: (self.totals[a] / self.counts[a]
                           + self.c * np.sqrt(ln_t / self.counts[a])),
        )

    def update(self, alg, tps, reward):
        self.t += 1
        self.counts[alg] += 1
        self.totals[alg] += tps


class FixedProtocolSelector:
    """Always pick a single hard-coded protocol. Static baseline for
    Fig 5 task-adaptation — shows what a no-adaptation operator gets.
    """
    def __init__(self, all_algs, protocol):
        self.algs     = list(all_algs)
        self.protocol = protocol
        if protocol not in self.algs:
            raise ValueError(f"FixedProtocolSelector: {protocol!r} not in pool {self.algs}")

    def select(self):
        return self.protocol

    def update(self, alg, tps, reward):
        pass  # static — no learning


class BestFixedSelector:
    """Round-robin warmup, then stick with the best cumulative-mean protocol."""
    def __init__(self, all_algs, warmup_steps=None):
        self.algs    = list(all_algs)
        # Default: each protocol tried twice during warmup.
        self.warmup  = warmup_steps if warmup_steps is not None else 2 * len(self.algs)
        self.step    = 0
        self.history = defaultdict(list)

    def select(self):
        if self.step < self.warmup:
            return self.algs[self.step % len(self.algs)]
        return max(
            self.algs,
            key=lambda a: np.mean(self.history[a]) if self.history[a] else 0.0,
        )

    def update(self, alg, tps, reward):
        self.step += 1
        self.history[alg].append(tps)
