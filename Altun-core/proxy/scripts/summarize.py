#!/usr/bin/env python3
"""One CSV row per switching run: checker results, switch outcomes, pause and
certification latency, rejected attestations.

  summarize.py RUNDIR [RUNDIR ...] > summary.csv

RUNDIR is a run_switching.sh output directory (segments.jsonl, check.json).
Phase latencies are medians / percentiles over the directives of the run.
"""
import csv
import json
import os
import sys


def q(xs, p):
    if not xs:
        return None
    xs = sorted(xs)
    k = (len(xs) - 1) * p
    lo, hi = int(k), min(int(k) + 1, len(xs) - 1)
    return round(xs[lo] + (xs[hi] - xs[lo]) * (k - lo), 2)


def row_of(run):
    name = os.path.basename(run.rstrip('/'))
    parent = os.path.basename(os.path.dirname(run.rstrip('/')))
    C = json.load(open(os.path.join(run, 'check.json')))
    J = [json.loads(l) for l in open(os.path.join(run, 'segments.jsonl'))]
    sw = [j for j in J if j.get('kind') == 'switch']
    rel = {j['window']: j['t'] for j in J if j.get('kind') == 'released'}
    for j in sw:
        if 'released' not in j['phases'] and j['window'] in rel:
            j['phases']['released'] = rel[j['window']]
    ok = [j for j in sw if j.get('ok')]
    ab = [j for j in sw if j.get('aborted')]
    refused = [j for j in sw if not j.get('ok') and not j.get('aborted')]

    def ms(j, a, b):
        if a in j['phases'] and b in j['phases']:
            return (j['phases'][b] - j['phases'][a]) / 1e6
        return None

    def phase(js, a, b):
        return [ms(j, a, b) for j in js if ms(j, a, b) is not None]

    pause = phase(sw, 'stopped', 'released')
    cert = phase(sw, 'sealed', 'certified')
    cert_c2b = phase([j for j in sw if j['from'] == 'paxos'], 'sealed', 'certified')
    cert_b2c = phase([j for j in sw if j['from'] == 'hotstuff'], 'sealed', 'certified')
    act = phase(ok, 'imported', 'activated')
    act_b2c = phase([j for j in ok if j['to'] == 'paxos'], 'imported', 'activated')
    act_c2b = phase([j for j in ok if j['to'] == 'hotstuff'], 'imported', 'activated')
    seal = phase(sw, 'drained', 'sealed')
    att_rej = [j for j in J if j.get('kind') == 'attestation-rejected']
    return {
        'run': parent, 'scenario': name, 'ops': C['ops'], 'pending': C['pending'],
        'violations': C['violations_total'], 'lin_bad_keys': C['lin_violating_keys'],
        'dup_pos': C['dup_positions'], 'read_viol': C['read_violations'],
        'seg_viol': C.get('segment_violations'),
        'directives': len(sw),
        'ok_c2b': sum(1 for j in ok if j['to'] == 'hotstuff'),
        'ok_b2c': sum(1 for j in ok if j['to'] == 'paxos'),
        'aborted': len(ab), 'refused': len(refused),
        'cert_quorum': (sw[0].get('cert_quorum') if sw else None),
        'cert_matched_min': min((j.get('cert_matched', 0) for j in sw), default=None),
        'cert_matched_max': max((j.get('cert_matched', 0) for j in sw), default=None),
        'att_rejected_seal': sum(1 for j in att_rej if j.get('phase') == 'seal'),
        'att_rejected_start': sum(1 for j in att_rej if j.get('phase') == 'start'),
        'att_rejected_members': ','.join(sorted({j['member'] for j in att_rej})),
        'act_replies_rejected': sum(len(j.get('rejected') or []) for j in sw),
        'snapshots_rejected': sum(1 for j in J if j.get('kind') == 'snapshot-rejected'),
        'pause_med_ms': q(pause, .5), 'pause_p95_ms': q(pause, .95), 'pause_max_ms': q(pause, 1),
        'cert_med_ms': q(cert, .5), 'cert_p95_ms': q(cert, .95), 'cert_max_ms': q(cert, 1),
        'cert_c2b_med_ms': q(cert_c2b, .5), 'cert_b2c_med_ms': q(cert_b2c, .5),
        'seal_med_ms': q(seal, .5),
        'act_med_ms': q(act, .5), 'act_p95_ms': q(act, .95),
        'act_c2b_med_ms': q(act_c2b, .5), 'act_b2c_med_ms': q(act_b2c, .5),
        'reasons': '; '.join(sorted({(j.get('reason') or '')[:60] for j in sw if j.get('reason')})),
    }


def main():
    rows = [row_of(d) for d in sys.argv[1:]]
    cols = []
    for r in rows:
        for k in r:
            if k not in cols:
                cols.append(k)
    w = csv.DictWriter(sys.stdout, fieldnames=cols)
    w.writeheader()
    for r in rows:
        w.writerow(r)


if __name__ == '__main__':
    main()
