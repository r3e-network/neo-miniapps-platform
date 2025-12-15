#!/bin/bash
#
# cert-manager ClusterIssuer Configuration Helper
# Validates and updates email addresses for Let's Encrypt ACME registration
#
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
ISSUER_FILE="$PROJECT_ROOT/k8s/platform/cert-manager/cluster-issuer.yaml"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

validate_email() {
    local email="$1"
    if [[ ! "$email" =~ ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$ ]]; then
        return 1
    fi
    if [[ "$email" == *"@example.com" ]]; then
        return 1
    fi
    return 0
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Configure cert-manager ClusterIssuer email for Let's Encrypt ACME registration.

OPTIONS:
    --email <email>     Set email address for ACME registration
    --check             Check current configuration
    --apply             Apply configuration to Kubernetes cluster
    -h, --help          Show this help message

EXAMPLES:
    # Set email and apply to cluster
    $0 --email devops@yourdomain.com --apply

    # Just update the file without applying
    $0 --email devops@yourdomain.com

    # Check current configuration
    $0 --check

NOTES:
    - Email is required for Let's Encrypt ACME registration
    - You will receive expiration notices and important updates at this email
    - The email cannot be @example.com (placeholder)
    - Always test with letsencrypt-staging before switching to production

EOF
}

check_config() {
    log_info "Checking cert-manager email configuration..."

    if [[ ! -f "$ISSUER_FILE" ]]; then
        log_error "ClusterIssuer file not found: $ISSUER_FILE"
        exit 1
    fi

    if grep -q "email:.*@example\.com" "$ISSUER_FILE"; then
        log_error "ClusterIssuer contains placeholder email (@example.com)"
        log_error "Run: $0 --email your-email@yourdomain.com"
        return 1
    fi

    local staging_email=$(grep -A 10 "letsencrypt-staging" "$ISSUER_FILE" | grep "email:" | sed 's/.*email:\s*\${CERT_MANAGER_EMAIL:-\(.*\)}.*/\1/' | tr -d ' ')
    local prod_email=$(grep -A 10 "letsencrypt-prod" "$ISSUER_FILE" | grep "email:" | sed 's/.*email:\s*\${CERT_MANAGER_EMAIL:-\(.*\)}.*/\1/' | tr -d ' ')

    log_info "Current configuration:"
    log_info "  Staging email: $staging_email"
    log_info "  Production email: $prod_email"

    if validate_email "$staging_email"; then
        log_info "✓ Email configuration is valid"
        return 0
    else
        log_error "✗ Email configuration is invalid"
        return 1
    fi
}

set_email() {
    local email="$1"

    log_info "Validating email: $email"

    if ! validate_email "$email"; then
        log_error "Invalid email address: $email"
        log_error "Email must:"
        log_error "  - Follow standard email format"
        log_error "  - Not use @example.com domain"
        exit 1
    fi

    log_info "Updating ClusterIssuer configuration..."

    # Create backup
    cp "$ISSUER_FILE" "$ISSUER_FILE.bak"
    log_info "Created backup: $ISSUER_FILE.bak"

    # Update email in file
    sed -i "s/\${CERT_MANAGER_EMAIL:-[^}]*}/\${CERT_MANAGER_EMAIL:-$email}/g" "$ISSUER_FILE"

    log_info "✓ Email updated to: $email"
    log_info "File updated: $ISSUER_FILE"
}

apply_to_cluster() {
    log_info "Applying configuration to Kubernetes cluster..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi

    if ! kubectl get nodes &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    # Check if cert-manager is installed
    if ! kubectl get namespace cert-manager &> /dev/null; then
        log_warn "cert-manager namespace not found. Is cert-manager installed?"
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Aborted"
            exit 0
        fi
    fi

    # Apply with envsubst if CERT_MANAGER_EMAIL is set
    if [[ -n "${CERT_MANAGER_EMAIL:-}" ]]; then
        log_info "Using CERT_MANAGER_EMAIL from environment: $CERT_MANAGER_EMAIL"
        envsubst < "$ISSUER_FILE" | kubectl apply -f -
    else
        kubectl apply -f "$ISSUER_FILE"
    fi

    log_info "✓ ClusterIssuer configuration applied"
    log_info ""
    log_info "To verify:"
    log_info "  kubectl get clusterissuer"
    log_info "  kubectl describe clusterissuer letsencrypt-staging"
}

# Parse arguments
EMAIL=""
CHECK_ONLY=false
APPLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --email)
            EMAIL="$2"
            shift 2
            ;;
        --check)
            CHECK_ONLY=true
            shift
            ;;
        --apply)
            APPLY=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main logic
if [[ "$CHECK_ONLY" == "true" ]]; then
    check_config
    exit $?
fi

if [[ -z "$EMAIL" ]]; then
    log_error "Email address required"
    usage
    exit 1
fi

set_email "$EMAIL"

if [[ "$APPLY" == "true" ]]; then
    apply_to_cluster
else
    log_info ""
    log_info "Configuration updated but not applied to cluster"
    log_info "To apply, run: $0 --email $EMAIL --apply"
    log_info "Or manually: kubectl apply -f $ISSUER_FILE"
fi

log_info ""
log_info "Remember to test with letsencrypt-staging first!"
log_info "Switch to letsencrypt-prod only after successful testing."
