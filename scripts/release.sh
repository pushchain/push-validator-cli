#!/usr/bin/env bash
#
# Release script for push-validator-cli
# Creates a git tag and pushes it to trigger GitHub Actions release workflow
#
# Usage: ./scripts/release.sh <version>
#   Examples:
#     ./scripts/release.sh 0.2.0
#     ./scripts/release.sh v0.2.0
#     ./scripts/release.sh 0.2.0-beta.1
#
# Options:
#   --dry-run    Show what would be done without making changes
#   --force      Skip confirmation prompt
#   --help       Show this help message

set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────────────────

REPO_OWNER="pushchain"
REPO_NAME="push-validator-cli"
GITHUB_REPO="https://github.com/${REPO_OWNER}/${REPO_NAME}"

# ─────────────────────────────────────────────────────────────────────────────
# Colors and formatting
# ─────────────────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m' # No Color

# ─────────────────────────────────────────────────────────────────────────────
# Helper functions
# ─────────────────────────────────────────────────────────────────────────────

print_step() {
    echo -e "\n${BLUE}${BOLD}$1${NC}"
}

print_success() {
    echo -e "   ${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "   ${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "   ${RED}✗${NC} $1"
}

print_info() {
    echo -e "   ${DIM}$1${NC}"
}

show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS] <version>

Create a release tag and push it to GitHub to trigger the release workflow.

Arguments:
  version    Version number (e.g., 0.2.0 or v0.2.0)
             Supports semver with optional pre-release (e.g., 0.2.0-beta.1)

Options:
  --dry-run  Show what would be done without making changes
  --force    Skip confirmation prompt
  --help     Show this help message

Examples:
  $(basename "$0") 0.2.0           # Release v0.2.0
  $(basename "$0") v0.2.0          # Same as above (v prefix is optional)
  $(basename "$0") 0.2.0-rc.1      # Pre-release version
  $(basename "$0") --dry-run 0.2.0 # Show what would happen

EOF
    exit 0
}

# ─────────────────────────────────────────────────────────────────────────────
# Parse arguments
# ─────────────────────────────────────────────────────────────────────────────

DRY_RUN=false
FORCE=false
VERSION=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --help|-h)
            show_help
            ;;
        -*)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
        *)
            if [[ -z "$VERSION" ]]; then
                VERSION="$1"
            else
                echo "Error: Multiple versions specified"
                exit 1
            fi
            shift
            ;;
    esac
done

if [[ -z "$VERSION" ]]; then
    echo "Error: Version number required"
    echo "Use --help for usage information"
    exit 1
fi

# Normalize version (ensure v prefix)
VERSION="v${VERSION#v}"

# ─────────────────────────────────────────────────────────────────────────────
# Validation functions
# ─────────────────────────────────────────────────────────────────────────────

validate_version() {
    print_step "Validating version..."

    # Check semver format: vMAJOR.MINOR.PATCH[-PRERELEASE]
    if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
        print_error "Invalid version format: $VERSION"
        print_info "Expected format: vX.Y.Z or vX.Y.Z-prerelease"
        print_info "Examples: v0.2.0, v1.0.0-beta.1, v2.1.0-rc.1"
        exit 1
    fi

    print_success "Version: ${BOLD}$VERSION${NC}"
    print_success "Format: Valid semver"

    # Check if tag already exists
    if git rev-parse "$VERSION" >/dev/null 2>&1; then
        print_error "Tag $VERSION already exists!"
        print_info "Use a different version or delete the existing tag first"
        exit 1
    fi
    print_success "Tag does not exist yet"
}

check_git_status() {
    print_step "Checking git status..."

    # Check for uncommitted changes
    if ! git diff-index --quiet HEAD -- 2>/dev/null; then
        print_error "Uncommitted changes found"
        print_info "Please commit or stash your changes before releasing"
        git status --short
        exit 1
    fi
    print_success "Working directory is clean"

    # Check current branch
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$CURRENT_BRANCH" != "main" && "$CURRENT_BRANCH" != "master" ]]; then
        print_warning "Not on main branch (currently on: $CURRENT_BRANCH)"
        if [[ "$FORCE" != true && "$DRY_RUN" != true ]]; then
            read -p "   Continue anyway? (y/N) " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    else
        print_success "Current branch: $CURRENT_BRANCH"
    fi

    # Check if we're up to date with remote
    git fetch origin --tags --quiet 2>/dev/null || true

    LOCAL_COMMIT=$(git rev-parse HEAD)
    REMOTE_COMMIT=$(git rev-parse "origin/$CURRENT_BRANCH" 2>/dev/null || echo "")

    if [[ -n "$REMOTE_COMMIT" && "$LOCAL_COMMIT" != "$REMOTE_COMMIT" ]]; then
        print_warning "Local branch differs from remote"
        print_info "Consider pushing or pulling before releasing"
    else
        print_success "Branch is up to date with remote"
    fi
}

generate_changelog() {
    print_step "Generating changelog..."

    # Get the previous tag
    LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

    if [[ -n "$LAST_TAG" ]]; then
        print_success "Previous release: $LAST_TAG"

        # Count commits since last tag
        COMMIT_COUNT=$(git rev-list "$LAST_TAG"..HEAD --count)
        print_success "Commits since $LAST_TAG: $COMMIT_COUNT"

        if [[ "$COMMIT_COUNT" -eq 0 ]]; then
            print_warning "No new commits since $LAST_TAG"
            if [[ "$FORCE" != true && "$DRY_RUN" != true ]]; then
                read -p "   Continue anyway? (y/N) " -n 1 -r
                echo
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    exit 1
                fi
            fi
        fi

        # Generate changelog
        CHANGELOG=$(git log --pretty=format:"- %s (%h)" "$LAST_TAG"..HEAD 2>/dev/null || echo "")
    else
        print_info "No previous tags found (first release)"
        COMMIT_COUNT=$(git rev-list HEAD --count)
        CHANGELOG=$(git log --pretty=format:"- %s (%h)" 2>/dev/null || echo "")
    fi

    echo ""
    echo -e "   ${DIM}─────────────────────────────────────────${NC}"
    if [[ -n "$CHANGELOG" ]]; then
        echo "$CHANGELOG" | head -20 | while IFS= read -r line; do
            echo -e "   ${DIM}$line${NC}"
        done

        if [[ $(echo "$CHANGELOG" | wc -l) -gt 20 ]]; then
            echo -e "   ${DIM}... and $(($(echo "$CHANGELOG" | wc -l) - 20)) more commits${NC}"
        fi
    else
        echo -e "   ${DIM}(no commits)${NC}"
    fi
    echo -e "   ${DIM}─────────────────────────────────────────${NC}"
}

show_summary() {
    print_step "Release Summary"

    echo ""
    echo -e "   ${BOLD}Version:${NC}   $VERSION"
    echo -e "   ${BOLD}Previous:${NC}  ${LAST_TAG:-"(none)"}"
    echo -e "   ${BOLD}Commits:${NC}   $COMMIT_COUNT"
    echo -e "   ${BOLD}Branch:${NC}    $CURRENT_BRANCH"
    echo -e "   ${BOLD}Commit:${NC}    $(git rev-parse --short HEAD)"
    echo ""

    if [[ "$DRY_RUN" == true ]]; then
        echo -e "   ${YELLOW}${BOLD}DRY RUN${NC} - No changes will be made"
        echo ""
    fi
}

confirm_release() {
    if [[ "$DRY_RUN" == true ]]; then
        return 0
    fi

    if [[ "$FORCE" == true ]]; then
        return 0
    fi

    echo -e -n "${CYAN}${BOLD}Create and push tag $VERSION?${NC} (y/N) "
    read -n 1 -r
    echo

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Release cancelled"
        exit 0
    fi
}

create_and_push_tag() {
    if [[ "$DRY_RUN" == true ]]; then
        print_step "[DRY RUN] Would create and push tag"
        print_info "git tag -a $VERSION -m \"Release $VERSION\" -m \"<changelog>\""
        print_info "git push origin $VERSION"
        return 0
    fi

    print_step "Creating tag..."

    # Create annotated tag with changelog as message
    TAG_MESSAGE="Release $VERSION

Changes since ${LAST_TAG:-"initial"}:
$CHANGELOG"

    git tag -a "$VERSION" -m "$TAG_MESSAGE"
    print_success "Created tag $VERSION"

    print_step "Pushing tag to origin..."
    git push origin "$VERSION"
    print_success "Pushed tag $VERSION"
}

show_release_info() {
    echo ""
    echo -e "${GREEN}${BOLD}Release triggered successfully!${NC}"
    echo ""
    echo -e "   ${BOLD}GitHub Actions:${NC}"
    echo -e "   ${CYAN}${GITHUB_REPO}/actions${NC}"
    echo ""
    echo -e "   ${BOLD}Release page (after workflow completes):${NC}"
    echo -e "   ${CYAN}${GITHUB_REPO}/releases/tag/${VERSION}${NC}"
    echo ""

    if [[ "$DRY_RUN" != true ]]; then
        print_info "The release workflow will build binaries for:"
        print_info "  - Linux (amd64, arm64)"
        print_info "  - macOS (amd64, arm64)"
        echo ""
    fi
}

# ─────────────────────────────────────────────────────────────────────────────
# Main workflow
# ─────────────────────────────────────────────────────────────────────────────

main() {
    echo ""
    echo -e "${BOLD}Push Validator CLI - Release Script${NC}"
    echo -e "${DIM}═════════════════════════════════════${NC}"

    validate_version
    check_git_status
    generate_changelog
    show_summary
    confirm_release
    create_and_push_tag
    show_release_info
}

main
