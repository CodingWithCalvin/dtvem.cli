#!/bin/bash
set -e

# dtvem uninstaller for macOS and Linux
# Usage:
#   Standard:  curl -fsSL https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.sh | bash
#   No prompt: curl -fsSL https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.sh | bash -s -- --yes
#
# Removes everything dtvem puts on a machine:
#   - the dtvem directory: the dtvem binaries, shims, every runtime version
#     installed with dtvem (and its global packages), config and cache
#   - the PATH lines the installer and `dtvem init` added to shell config files
# Project files (.dtvem/runtimes.json, .tool-versions, .nvmrc, ...) are left alone.

ASSUME_YES=false

usage() {
    echo "Usage: uninstall.sh [--yes]"
    echo ""
    echo "Removes dtvem, every runtime version installed with it, and the PATH"
    echo "lines it added to your shell config files."
    echo ""
    echo "Options:"
    echo "  -y, --yes   Uninstall without asking for confirmation"
    echo "  -h, --help  Show this help"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -y|--yes)
            ASSUME_YES=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() {
    echo -e "${CYAN}→${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Shell config files the installer and `dtvem init` add PATH lines to
SHELL_CONFIGS=(
    "$HOME/.bashrc"
    "$HOME/.bash_profile"
    "$HOME/.profile"
    "$HOME/.zshrc"
    "$HOME/.config/fish/config.fish"
)

# dtvem roots found on this machine (see find_roots)
ROOTS=()

# PATH lines found by scan_config, as "<file>: <line>"
CONFIG_BLOCKS=()

# Prints a path without trailing slashes
strip_slashes() {
    local path=$1
    while [ "${#path}" -gt 1 ] && [ "${path%/}" != "$path" ]; do
        path=${path%/}
    done
    echo "$path"
}

# Prints a directory with symlinks resolved
real_dir() {
    (cd "$1" 2>/dev/null && pwd -P)
}

# Whether a root is a directory dtvem owns outright, named dtvem or .dtvem.
# A DTVEM_ROOT with any other name may hold other files, so only the
# entries dtvem creates in it are removed.
owned_root() {
    case "$(basename "$1")" in
        dtvem|.dtvem)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Prints the paths that removing a root deletes, one per line
root_entries() {
    local root=$1 entry

    if owned_root "$root"; then
        if [ -e "$root" ]; then
            echo "$root"
        fi
        return 0
    fi

    for entry in shims versions config cache tmp bin/dtvem bin/dtvem-shim; do
        if [ -e "$root/$entry" ] || [ -L "$root/$entry" ]; then
            echo "$root/$entry"
        fi
    done
}

# Finds every dtvem root on this machine: DTVEM_ROOT, XDG_DATA_HOME/dtvem,
# ~/.local/share/dtvem (the Linux default) and ~/.dtvem (the macOS default,
# and where Linux installs lived before dtvem followed XDG).
find_roots() {
    local candidate dir real existing duplicate home_real

    home_real=$(real_dir "$HOME") || home_real=$HOME

    for candidate in "$DTVEM_ROOT" "${XDG_DATA_HOME:+$XDG_DATA_HOME/dtvem}" "$HOME/.local/share/dtvem" "$HOME/.dtvem"; do
        if [ -z "$candidate" ]; then
            continue
        fi
        dir=$(strip_slashes "$candidate")
        if [ ! -d "$dir" ]; then
            continue
        fi

        real=$(real_dir "$dir") || real=$dir
        if [ "$real" = "/" ] || [ "$real" = "$home_real" ]; then
            warning "Skipping $dir: it is / or your home directory"
            continue
        fi

        duplicate=false
        for existing in "${ROOTS[@]}"; do
            if [ "$(real_dir "$existing")" = "$real" ]; then
                duplicate=true
            fi
        done
        if [ "$duplicate" = true ]; then
            continue
        fi

        if [ -n "$(root_entries "$dir")" ]; then
            ROOTS+=("$dir")
        fi
    done
}

# Prints the directory a PATH line written by dtvem puts on PATH: the first
# double-quoted string, up to any ":". dtvem writes
#   export PATH="<dir>:$PATH"       (bash, zsh, sh)
#   set -gx PATH "<dir>" $PATH      (fish)
path_line_dir() {
    local line=${1%$'\r'} dir

    case $line in
        *PATH*\"*\"*)
            ;;
        *)
            return 1
            ;;
    esac

    dir=${line#*\"}
    dir=${dir%%\"*}
    dir=${dir%%:*}
    case $dir in
        \$HOME/*|\$\{HOME\}/*|\~/*)
            dir="$HOME/${dir#*/}"
            ;;
    esac

    if [ -z "$dir" ]; then
        return 1
    fi
    strip_slashes "$dir"
}

# Whether a directory dtvem put on PATH should come off it: it is in a root
# being removed, or it is gone and was in a dtvem directory (left behind by
# an earlier install). Another install's existing directory is kept.
removable_dir() {
    local dir=$1 root

    for root in "${ROOTS[@]}"; do
        case $dir in
            "$root"|"$root"/*)
                return 0
                ;;
        esac
    done

    case "/$dir/" in
        */dtvem/*|*/.dtvem/*)
            [ ! -e "$dir" ]
            ;;
        *)
            return 1
            ;;
    esac
}

# scan_config FILE APPLY
#
# Finds the blocks dtvem wrote to a shell config file: a "# Added by dtvem"
# or "# Added by dtvem installer" line followed by the PATH line it added.
# Blocks whose PATH line names a directory being removed (see
# removable_dir) are added to CONFIG_BLOCKS. With APPLY=true the file is
# rewritten without them, and without the blank line dtvem wrote above each.
# Lines without the marker above them are never touched.
scan_config() {
    local file=$1 apply=$2
    local marker='^[[:space:]]*# Added by dtvem( installer)?[[:space:]]*$'
    local blank='^[[:space:]]*$'
    local line next dir i n last found=0
    local -a lines out

    if [ ! -f "$file" ]; then
        return 0
    fi

    lines=()
    while IFS= read -r line || [ -n "$line" ]; do
        lines+=("$line")
    done < "$file"

    out=()
    n=${#lines[@]}
    i=0
    while [ "$i" -lt "$n" ]; do
        line=${lines[$i]}
        if [[ $line =~ $marker ]] && [ $((i + 1)) -lt "$n" ]; then
            next=${lines[$((i + 1))]}
            if dir=$(path_line_dir "$next") && removable_dir "$dir"; then
                CONFIG_BLOCKS+=("$file: ${next%$'\r'}")
                found=$((found + 1))
                last=$((${#out[@]} - 1))
                if [ "$last" -ge 0 ] && [[ ${out[$last]} =~ $blank ]]; then
                    unset "out[$last]"
                fi
                i=$((i + 2))
                continue
            fi
        fi
        out+=("$line")
        i=$((i + 1))
    done

    if [ "$apply" != true ] || [ "$found" -eq 0 ]; then
        return 0
    fi

    # Write in place (not via a temp file and mv) so a config file that is
    # a symlink, such as one managed in a dotfiles repo, stays a symlink.
    if [ "${#out[@]}" -eq 0 ]; then
        : > "$file" || return 1
    else
        printf '%s\n' "${out[@]}" > "$file" || return 1
    fi
}

# Prints the runtime versions installed under a root, one runtime per line
installed_runtimes() {
    local root=$1 runtime_dir version_dir versions

    for runtime_dir in "$root"/versions/*/; do
        if [ ! -d "$runtime_dir" ]; then
            continue
        fi
        versions=""
        for version_dir in "$runtime_dir"*/; do
            if [ -d "$version_dir" ]; then
                versions="$versions $(basename "$version_dir")"
            fi
        done
        if [ -n "$versions" ]; then
            echo "$(basename "$runtime_dir"):$versions"
        fi
    done
}

# Describes what removing a root deletes
show_root() {
    local root=$1 entry runtimes

    if owned_root "$root"; then
        echo "  $root"
    else
        echo "  dtvem files in $root:"
        while IFS= read -r entry; do
            echo "    $entry"
        done < <(root_entries "$root")
    fi

    runtimes=$(installed_runtimes "$root")
    if [ -n "$runtimes" ]; then
        echo "    Installed runtimes, with their global packages:"
        while IFS= read -r entry; do
            echo "      $entry"
        done <<< "$runtimes"
    fi
    echo ""
}

# Deletes a path, making read-only directories writable if rm can't get
# through them. Neither rm -rf nor chmod -R follows symlinks, so a link to a
# directory outside the root (such as an `npm link`) is removed, not its target.
delete_path() {
    if rm -rf "$1" 2>/dev/null; then
        return 0
    fi
    chmod -R u+w "$1" 2>/dev/null || true
    rm -rf "$1"
}

# Removes a root (see root_entries), and reports whether it is gone
remove_root() {
    local root=$1 entry

    while IFS= read -r entry; do
        delete_path "$entry" || true
    done < <(root_entries "$root")

    if ! owned_root "$root"; then
        rmdir "$root/bin" 2>/dev/null || true
        rmdir "$root" 2>/dev/null || true
    fi

    [ -z "$(root_entries "$root")" ]
}

# Asks whether to go ahead, on the terminal: piped into bash, stdin is the
# script itself
confirm() {
    local reply

    if [ "$ASSUME_YES" = true ]; then
        return 0
    fi

    if ! (: < /dev/tty) 2>/dev/null; then
        error "There is no terminal to ask for confirmation on"
        info "To uninstall without being asked, run:"
        echo "    curl -fsSL https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/uninstall.sh | bash -s -- --yes"
        exit 1
    fi

    printf "Remove dtvem? [y/N] "
    read -r reply < /dev/tty || reply=""
    echo ""
    case $reply in
        y|Y|yes|Yes|YES)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Reports what is still around that this script leaves alone
report_leftovers() {
    local config matches line other found=false

    for config in "${SHELL_CONFIGS[@]}"; do
        if [ ! -f "$config" ]; then
            continue
        fi
        if ! matches=$(grep -in 'dtvem' "$config" 2>/dev/null); then
            continue
        fi
        if [ "$found" = false ]; then
            echo ""
            warning "These lines in your shell config still mention dtvem and were left alone."
            info "Remove them if you no longer need them:"
            found=true
        fi
        while IFS= read -r line; do
            echo "    $config:$line"
        done <<< "$matches"
    done

    other=$(command -v dtvem 2>/dev/null) || other=""
    if [ -n "$other" ]; then
        echo ""
        warning "Another dtvem is still on your PATH: $other"
        info "It wasn't installed by the dtvem installer. Remove it the way you installed it."
    fi

    if [ -n "$DTVEM_ROOT" ]; then
        echo ""
        info "DTVEM_ROOT is still set in your environment. Unset it where you set it."
    fi
}

main() {
    local root block config before removed failed=false

    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}   dtvem uninstaller${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    find_roots
    for config in "${SHELL_CONFIGS[@]}"; do
        scan_config "$config" false
    done

    if [ "${#ROOTS[@]}" -eq 0 ] && [ "${#CONFIG_BLOCKS[@]}" -eq 0 ]; then
        success "dtvem is not installed: there is nothing to remove"
        report_leftovers
        echo ""
        return 0
    fi

    info "This will remove:"
    echo ""
    for root in "${ROOTS[@]}"; do
        show_root "$root"
    done
    if [ "${#CONFIG_BLOCKS[@]}" -gt 0 ]; then
        echo "  PATH lines added by dtvem:"
        for block in "${CONFIG_BLOCKS[@]}"; do
            echo "    $block"
        done
        echo ""
    fi

    if ! confirm; then
        info "Canceled: nothing was removed"
        return 0
    fi

    for root in "${ROOTS[@]}"; do
        if ! remove_root "$root"; then
            error "Could not remove everything in $root"
            failed=true
        elif [ -e "$root" ]; then
            success "Removed dtvem from $root (other files in it were left alone)"
        else
            success "Removed $root"
        fi
    done

    CONFIG_BLOCKS=()
    for config in "${SHELL_CONFIGS[@]}"; do
        before=${#CONFIG_BLOCKS[@]}
        if scan_config "$config" true; then
            removed=$((${#CONFIG_BLOCKS[@]} - before))
            if [ "$removed" -gt 0 ]; then
                success "Removed $removed PATH line(s) from $config"
            fi
        else
            error "Could not update $config"
            failed=true
        fi
    done

    report_leftovers

    echo ""
    if [ "$failed" = true ]; then
        error "dtvem was not completely removed. Fix the errors above and run the uninstaller again."
        echo ""
        exit 1
    fi

    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}   dtvem has been uninstalled${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    info "Restart your terminal so the PATH change takes effect."
    info "Version files in your projects (.dtvem/runtimes.json, .tool-versions, .nvmrc,"
    info ".python-version, .ruby-version) were left in place."
    echo ""
}

main
