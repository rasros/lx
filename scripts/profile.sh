#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/profile.sh [output_dir] [linux_root_override]

Profiles lx across multiple repository scenarios and writes pprof top reports to:
  <output_dir>/raw
  <output_dir>/text

Repository bootstrap (reused if already present):
  <output_dir>/repos/linux       (https://github.com/torvalds/linux.git)
  <output_dir>/repos/linguist    (https://github.com/github-linguist/linguist.git)
  <output_dir>/archives          (generated from Linux subdirectories)

Arguments:
  output_dir           Directory for generated artifacts (default: profiles)
  linux_root_override  Optional Linux tree path to profile instead of cloned repo

Scenarios:
  - Code repos (fixed subdirs): linux/kernel and linguist/samples with -Y -u
  - Filtered walk mode: linguist/samples with -i and -e
  - Generated archive corpus: -Z on corpus dir and each archive file type
  - Every repo: -n0 and -t
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-profiles}"
LINUX_ROOT_OVERRIDE="${2:-}"

if [[ "$OUT_DIR" != /* ]]; then
  OUT_DIR="$ROOT_DIR/$OUT_DIR"
fi

if [[ -n "$LINUX_ROOT_OVERRIDE" && "$LINUX_ROOT_OVERRIDE" != /* ]]; then
  LINUX_ROOT_OVERRIDE="$ROOT_DIR/$LINUX_ROOT_OVERRIDE"
fi

RAW_DIR="$OUT_DIR/raw"
TEXT_DIR="$OUT_DIR/text"
REPOS_DIR="$OUT_DIR/repos"

LINUX_REPO_DIR="$REPOS_DIR/linux"
LINGUIST_REPO_DIR="$REPOS_DIR/linguist"
ARCHIVE_CORPUS_DIR="$OUT_DIR/archives"

LINUX_REPO_URL="https://github.com/torvalds/linux.git"
LINGUIST_REPO_URL="https://github.com/github-linguist/linguist.git"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "required command not found: $cmd" >&2
    exit 1
  fi
}

ensure_repo() {
  local name="$1"
  local url="$2"
  local dest="$3"

  if [[ -d "$dest/.git" ]]; then
    echo "Reusing existing repo: $name ($dest)"
    return 0
  fi

  if [[ -e "$dest" ]]; then
    echo "path exists but is not a git repo: $dest" >&2
    exit 1
  fi

  echo "Cloning $name into $dest"
  git clone --depth 1 "$url" "$dest"
}

repo_head() {
  local dest="$1"
  if [[ -d "$dest/.git" ]]; then
    git -C "$dest" rev-parse --short HEAD 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

repo_origin() {
  local dest="$1"
  if [[ -d "$dest/.git" ]]; then
    git -C "$dest" remote get-url origin 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

archive_scenario_name() {
  local path="$1"
  local base
  base="$(basename "$path")"
  base="${base//./_}"
  base="${base//-/_}"
  echo "archives_expand_${base}"
}

run_scenario() {
  local name="$1"
  shift
  local cpu_profile="$RAW_DIR/${name}.cpu.pprof"
  local mem_profile="$RAW_DIR/${name}.mem.pprof"
  local output_file="$RAW_DIR/${name}.lx.out"
  local stderr_log="$RAW_DIR/${name}.stderr.log"
  local cpu_text="$TEXT_DIR/${name}.cpu.top.txt"
  local mem_text="$TEXT_DIR/${name}.mem.top.txt"
  local -a cmd=("$LX_BIN" "--cpuprofile" "$cpu_profile" "--memprofile" "$mem_profile" "-o" "$output_file")

  cmd+=("$@")

  echo "Running $name"
  "${cmd[@]}" 2>"$stderr_log"

  go tool pprof -top "$LX_BIN" "$cpu_profile" >"$cpu_text"
  go tool pprof -top "$LX_BIN" "$mem_profile" >"$mem_text"

  {
    echo
    echo "[$name]"
    printf 'command:'
    printf ' %q' "${cmd[@]}"
    echo
    echo "cpu profile: $cpu_profile"
    echo "mem profile: $mem_profile"
    echo "lx output: $output_file"
    echo "stderr log: $stderr_log"
    echo "cpu report: $cpu_text"
    echo "mem report: $mem_text"
  } >>"$SCENARIO_FILE"
}

build_archive_corpus() {
  local linux_root="$1"
  local out_dir="$2"
  local -a source_subdirs=("kernel" "drivers" "tools" "Documentation")

  for subdir in "${source_subdirs[@]}"; do
    if [[ ! -d "$linux_root/$subdir" ]]; then
      echo "required archive source subdir missing: $linux_root/$subdir" >&2
      exit 1
    fi
  done

  rm -rf "$out_dir"
  mkdir -p "$out_dir"

  (
    cd "$linux_root"
    zip -qr "$out_dir/kernel.zip" "kernel"
    zip -qr "$out_dir/tools.zip" "tools"
    tar -cf "$out_dir/documentation.tar" "Documentation"
    tar -czf "$out_dir/drivers.tar.gz" "drivers"
    tar -cjf "$out_dir/kernel.tar.bz2" "kernel"
    tar -czf "$out_dir/tools_drivers.tar.gz" "tools" "drivers"
  )
}

require_cmd git
require_cmd go
require_cmd tar
require_cmd zip

mkdir -p "$RAW_DIR" "$TEXT_DIR" "$REPOS_DIR"

ensure_repo "linux" "$LINUX_REPO_URL" "$LINUX_REPO_DIR"
ensure_repo "linguist" "$LINGUIST_REPO_URL" "$LINGUIST_REPO_DIR"

if [[ -n "$LINUX_ROOT_OVERRIDE" ]]; then
  LINUX_ROOT="$LINUX_ROOT_OVERRIDE"
else
  LINUX_ROOT="$LINUX_REPO_DIR"
fi

if [[ ! -d "$LINUX_ROOT" ]]; then
  echo "linux_root does not exist: $LINUX_ROOT" >&2
  exit 1
fi

build_archive_corpus "$LINUX_ROOT" "$ARCHIVE_CORPUS_DIR"

LX_BIN="$OUT_DIR/lx-profile-bin"
SCENARIO_FILE="$TEXT_DIR/00_scenarios.txt"

echo "Building lx binary at $LX_BIN"
(
  cd "$ROOT_DIR"
  go build -o "$LX_BIN" ./cmd/lx
)

cat >"$SCENARIO_FILE" <<EOF
lx profiling scenarios
repository: $ROOT_DIR
output dir: $OUT_DIR
linux profile path: $LINUX_ROOT

repositories:
linux dir: $LINUX_REPO_DIR
linux origin: $(repo_origin "$LINUX_REPO_DIR")
linux head: $(repo_head "$LINUX_REPO_DIR")
linguist dir: $LINGUIST_REPO_DIR
linguist origin: $(repo_origin "$LINGUIST_REPO_DIR")
linguist head: $(repo_head "$LINGUIST_REPO_DIR")
archives dir: $ARCHIVE_CORPUS_DIR
archives source root: $LINUX_ROOT
EOF

{
  echo "archives files:"
  for f in "$ARCHIVE_CORPUS_DIR"/*; do
    echo "  - $(basename "$f")"
  done
} >>"$SCENARIO_FILE"

CODE_REPO_NAMES=("linux" "linguist")
CODE_REPO_PATHS=("$LINUX_ROOT" "$LINGUIST_REPO_DIR")
CODE_REPO_SKELETON_SUBDIRS=("kernel" "samples")
FILTER_REPO_PATH="$LINGUIST_REPO_DIR/samples"
FILTER_INCLUDE_PATTERN="**/*.rb"
FILTER_EXCLUDE_PATTERN="**/*_test.rb"

ALL_REPO_NAMES=("linux" "linguist" "archives")
ALL_REPO_PATHS=("$LINUX_ROOT" "$LINGUIST_REPO_DIR" "$ARCHIVE_CORPUS_DIR")

for i in "${!CODE_REPO_NAMES[@]}"; do
  name="${CODE_REPO_NAMES[$i]}"
  path="${CODE_REPO_PATHS[$i]}"
  subdir="${CODE_REPO_SKELETON_SUBDIRS[$i]}"
  subdir_path="$path/$subdir"
  if [[ ! -d "$subdir_path" ]]; then
    echo "required skeleton subdir missing: $subdir_path" >&2
    exit 1
  fi
  echo "${name} skeleton subdir: $subdir_path" >>"$SCENARIO_FILE"
  run_scenario "${name}_skeleton_subdir" -Y -u "$subdir_path"
done

if [[ ! -d "$FILTER_REPO_PATH" ]]; then
  echo "required filter scenario path missing: $FILTER_REPO_PATH" >&2
  exit 1
fi
{
  echo "filter scenario path: $FILTER_REPO_PATH"
  echo "filter include: $FILTER_INCLUDE_PATTERN"
  echo "filter exclude: $FILTER_EXCLUDE_PATTERN"
} >>"$SCENARIO_FILE"
run_scenario "linguist_filters_include_exclude" -i "$FILTER_INCLUDE_PATTERN" -e "$FILTER_EXCLUDE_PATTERN" "$FILTER_REPO_PATH"

run_scenario "archives_expand_dir" -Z "$ARCHIVE_CORPUS_DIR"

while IFS= read -r archive_path; do
  scenario_name="$(archive_scenario_name "$archive_path")"
  run_scenario "$scenario_name" -Z "$archive_path"
done < <(find "$ARCHIVE_CORPUS_DIR" -maxdepth 1 -type f | sort)

for i in "${!ALL_REPO_NAMES[@]}"; do
  name="${ALL_REPO_NAMES[$i]}"
  path="${ALL_REPO_PATHS[$i]}"
  run_scenario "${name}_compact" -n0 "$path"
  run_scenario "${name}_tree" -t "$path"
done

echo "Done. Artifacts are in: $OUT_DIR"
