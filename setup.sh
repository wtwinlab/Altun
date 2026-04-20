#!/bin/bash
# setup.sh — Clone Altun-core and Altun_RL into this outer repo.
# Run once after: git clone https://github.com/Marvinmw/Altun.git
#
# Background: Altun-core/ and Altun_RL/ are NOT git submodules. They are
# two separate clones placed under this directory for convenience. The
# outer repo's .gitignore excludes both paths, so the three git indices
# never overlap — each repo is maintained via its own commit helper
# (commit_altun.sh / commit_altun_core.sh / commit_altun_rl.sh) or
# the unified commit_all.sh.
#
# Safe to re-run: if a sub-repo already exists, it is fetched and
# fast-forwarded on the required branch rather than re-cloned.
#
# Build / Python-env setup is handled per sub-repo:
#   Altun-core  — bash bin/build.sh      (NOT `make build` — that target
#                                         builds hotstuff/veritas only)
#   Altun_RL    — see Altun_RL README / conda env `rl`

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ALTUN_CORE="${SCRIPT_DIR}/Altun-core"
ALTUN_RL="${SCRIPT_DIR}/Altun_RL"
ADAPT_DIR="${SCRIPT_DIR}/baselines/ADAPT"
EXP_BRANCH="exp-new-experiments"
ADAPT_BRANCH="main"
CORE_REMOTE="git@github.com:wtwinlab/Altun-core.git"
RL_REMOTE="git@github.com:wtwinlab/Altun_RL.git"
ADAPT_REMOTE="git@github.com:wtwinlab/ADAPT.git"

clone_or_update() {
    local dir=$1 remote=$2 branch=$3 name
    name=$(basename "$dir")
    if [ -d "${dir}/.git" ]; then
        echo "[$name] already cloned — pulling $branch..."
        git -C "$dir" fetch origin
        git -C "$dir" checkout "$branch" 2>/dev/null || \
            git -C "$dir" checkout -b "$branch" origin/"$branch"
        git -C "$dir" pull origin "$branch"
    else
        echo "[$name] cloning $remote ($branch)..."
        rm -rf "$dir"
        git clone --branch "$branch" "$remote" "$dir"
    fi
    echo "[$name] ok — branch: $(git -C "$dir" branch --show-current)"
}

clone_or_update "$ALTUN_CORE" "$CORE_REMOTE"  "$EXP_BRANCH"
clone_or_update "$ALTUN_RL"   "$RL_REMOTE"    "$EXP_BRANCH"
mkdir -p "$(dirname "$ADAPT_DIR")"
clone_or_update "$ADAPT_DIR"  "$ADAPT_REMOTE" "$ADAPT_BRANCH"

echo ""
echo "Done. Next steps:"
echo "  Build Altun-core : cd Altun-core && bash bin/build.sh"
echo "  Setup Python env : cd Altun_RL   && conda env create / activate rl"
echo "  Sync design docs : bash sync_exp_design.sh  # outer -> sub-repos"
