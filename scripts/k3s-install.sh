#!/usr/bin/env bash
#
# k3s 集群初始化脚本 (STORY-1.1)
# 用途：在 Ubuntu 24.04 SGX VM 上安装 k3s 并配置 Intel SGX device plugin
# 幂等性：可重复执行
#

set -euo pipefail

# ==================== 配置参数 ====================
K3S_VERSION="${K3S_VERSION:-v1.28.5+k3s1}"
INSTALL_TIMEOUT="${INSTALL_TIMEOUT:-300}"
LOG_FILE="/var/log/k3s-install.log"
STATE_FILE="/var/lib/k3s-install-state"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ==================== 日志函数 ====================
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARN:${NC} $*" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR:${NC} $*" | tee -a "$LOG_FILE"
    exit 1
}

# ==================== 状态管理 ====================
save_state() {
    local step="$1"
    echo "$step" > "$STATE_FILE"
    log "✓ 检查点: $step"
}

get_state() {
    if [[ -f "$STATE_FILE" ]]; then
        cat "$STATE_FILE"
    else
        echo "none"
    fi
}

rollback_installation() {
    error "安装失败，执行回滚操作..."

    # 停止并卸载 k3s
    if command -v k3s-uninstall.sh &> /dev/null; then
        log "正在卸载 k3s..."
        k3s-uninstall.sh || warn "k3s 卸载失败"
    fi

    # 清理状态文件
    rm -f "$STATE_FILE"

    # 清理 kubeconfig
    if [[ -n "${SUDO_USER:-}" ]]; then
        local user_home=$(eval echo ~$SUDO_USER)
        rm -rf "$user_home/.kube"
    fi

    log "回滚完成，请检查日志: $LOG_FILE"
    exit 1
}

# ==================== 前置检查 ====================
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "此脚本必须以 root 权限运行: sudo $0"
    fi
}

check_os() {
    if ! grep -q "Ubuntu 24.04" /etc/os-release 2>/dev/null; then
        warn "未检测到 Ubuntu 24.04，但继续执行。当前系统: $(cat /etc/os-release | grep PRETTY_NAME)"
    fi
}

check_sgx_support() {
    log "检查 Intel SGX 支持..."

    if [[ ! -c /dev/sgx_enclave ]] && [[ ! -c /dev/sgx/enclave ]]; then
        warn "未检测到 SGX enclave 设备 (/dev/sgx_enclave 或 /dev/sgx/enclave)"
        warn "请确认已安装 SGX driver: https://github.com/intel/linux-sgx-driver"
    else
        log "✓ SGX enclave 设备已就绪"
    fi

    if [[ ! -c /dev/sgx_provision ]] && [[ ! -c /dev/sgx/provision ]]; then
        warn "未检测到 SGX provision 设备"
    else
        log "✓ SGX provision 设备已就绪"
    fi
}

check_resources() {
    log "检查系统资源..."

    local cpu_cores=$(nproc)
    local mem_gb=$(free -g | awk '/^Mem:/{print $2}')
    local disk_gb=$(df -BG / | awk 'NR==2 {print $4}' | sed 's/G//')

    log "CPU 核心: $cpu_cores, 内存: ${mem_gb}GB, 可用磁盘: ${disk_gb}GB"

    if [[ $cpu_cores -lt 8 ]]; then
        warn "CPU 核心数不足 8 核 (当前: $cpu_cores)，资源可能不足"
    fi

    if [[ $mem_gb -lt 28 ]]; then
        warn "内存不足 32GB (当前: ${mem_gb}GB)，资源可能不足"
    fi

    if [[ $disk_gb -lt 200 ]]; then
        warn "可用磁盘空间不足 256GB (当前: ${disk_gb}GB)"
    fi
}

# ==================== k3s 安装 ====================
install_k3s() {
    log "检查 k3s 是否已安装..."

    local current_state=$(get_state)
    if [[ "$current_state" == "k3s_installed" ]] || command -v k3s &> /dev/null; then
        local installed_version=$(k3s --version | head -1 | awk '{print $3}')
        log "k3s 已安装 (版本: $installed_version)"

        if systemctl is-active --quiet k3s; then
            log "k3s 服务正在运行，跳过安装"
            save_state "k3s_installed"
            return 0
        else
            warn "k3s 已安装但服务未运行，尝试启动..."
            systemctl start k3s
            save_state "k3s_installed"
            return 0
        fi
    fi

    log "开始安装 k3s $K3S_VERSION ..."

    # 禁用 Traefik (后续手动部署)
    # 禁用 ServiceLB (使用 ClusterIP)
    # 使用 Pod Security Standards (PSS) 替代废弃的 PodSecurityPolicy
    # PSS labels: privileged, baseline, restricted
    curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="$K3S_VERSION" sh -s - \
        --disable traefik \
        --disable servicelb \
        --write-kubeconfig-mode 644 \
        --kube-apiserver-arg "enable-admission-plugins=NodeRestriction,PodSecurity" \
        --kube-apiserver-arg "admission-control-config-file=/var/lib/rancher/k3s/server/pss-config.yaml" \
        || rollback_installation

    # 创建 Pod Security Standards 配置
    mkdir -p /var/lib/rancher/k3s/server
    cat > /var/lib/rancher/k3s/server/pss-config.yaml <<EOF
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1
    kind: PodSecurityConfiguration
    defaults:
      enforce: "baseline"
      enforce-version: "latest"
      audit: "restricted"
      audit-version: "latest"
      warn: "restricted"
      warn-version: "latest"
    exemptions:
      usernames: []
      runtimeClasses: []
      namespaces: [kube-system]
EOF

    save_state "k3s_installed"
    log "✓ k3s 安装完成"
}

wait_for_k3s() {
    log "等待 k3s 启动..."

    local timeout=$INSTALL_TIMEOUT
    local elapsed=0

    while ! kubectl get nodes &> /dev/null; do
        if [[ $elapsed -ge $timeout ]]; then
            rollback_installation
        fi

        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done

    echo ""
    log "✓ k3s API Server 已就绪"

    # 等待节点 Ready
    kubectl wait --for=condition=Ready nodes --all --timeout=120s \
        || rollback_installation

    save_state "k3s_ready"
    log "✓ 所有节点已 Ready"
}

# ==================== Namespace 创建 ====================
create_namespaces() {
    log "创建 Namespace: apps, platform, monitoring..."

    # 使用 Pod Security Standards labels
    # baseline: 限制已知的特权提升，允许默认配置
    # restricted: 严格限制，遵循当前 Pod 加固最佳实践
    kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: apps
  labels:
    name: apps
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
---
apiVersion: v1
kind: Namespace
metadata:
  name: platform
  labels:
    name: platform
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
---
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
  labels:
    name: monitoring
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
EOF

    save_state "namespaces_created"
    log "✓ Namespace 创建完成 (with Pod Security Standards)"
}

# ==================== ResourceQuota 配置 ====================
apply_resource_quotas() {
    log "配置 ResourceQuota (按架构文档精确值)..."

    # 架构文档 Section 4 精确值:
    # apps namespace: 15.0C request / 18.6C limit, 55.5Gi RAM request / 72Gi limit
    # platform namespace: 2.7C request / 4.2C limit, 8.4Gi RAM request / 10.5Gi limit
    # monitoring namespace: 1.5C request / 2.1C limit, 9.3Gi RAM request / 12Gi limit
    kubectl apply -f - <<EOF
apiVersion: v1
kind: ResourceQuota
metadata:
  name: apps-quota
  namespace: apps
spec:
  hard:
    requests.cpu: "15.0"
    requests.memory: "55.5Gi"
    limits.cpu: "18.6"
    limits.memory: "72Gi"
    persistentvolumeclaims: "20"
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: platform-quota
  namespace: platform
spec:
  hard:
    requests.cpu: "2.7"
    requests.memory: "8.4Gi"
    limits.cpu: "4.2"
    limits.memory: "10.5Gi"
    persistentvolumeclaims: "10"
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: monitoring-quota
  namespace: monitoring
spec:
  hard:
    requests.cpu: "1.5"
    requests.memory: "9.3Gi"
    limits.cpu: "2.1"
    limits.memory: "12Gi"
    persistentvolumeclaims: "10"
EOF

    save_state "quotas_applied"
    log "✓ ResourceQuota 配置完成 (与架构文档完全一致)"
}

# ==================== SGX Device Plugin ====================
install_sgx_device_plugin() {
    log "安装 Intel SGX Device Plugin..."

    # 检查是否已安装
    if kubectl get daemonset intel-sgx-plugin -n kube-system &> /dev/null; then
        log "SGX Device Plugin 已存在，跳过安装"
        return 0
    fi

    # 部署 Intel SGX Device Plugin
    kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: intel-sgx-plugin
  namespace: kube-system
  labels:
    app: intel-sgx-plugin
spec:
  selector:
    matchLabels:
      app: intel-sgx-plugin
  template:
    metadata:
      labels:
        app: intel-sgx-plugin
    spec:
      hostNetwork: true
      containers:
      - name: intel-sgx-plugin
        image: intel/intel-deviceplugin-sgx:v0.27.1
        imagePullPolicy: IfNotPresent
        securityContext:
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
        volumeMounts:
        - name: devfs
          mountPath: /dev
        - name: kubeletsockets
          mountPath: /var/lib/kubelet/device-plugins
      volumes:
      - name: devfs
        hostPath:
          path: /dev
      - name: kubeletsockets
        hostPath:
          path: /var/lib/kubelet/device-plugins
EOF

    log "✓ SGX Device Plugin 部署完成"

    # 等待 DaemonSet Ready
    sleep 10
    kubectl rollout status daemonset intel-sgx-plugin -n kube-system --timeout=60s \
        || warn "SGX Device Plugin 可能未完全就绪"
}

# ==================== kubeconfig 配置 ====================
setup_kubeconfig() {
    log "配置 kubeconfig..."

    # 为当前用户创建 kubeconfig (如果非 root 运行)
    if [[ -n "${SUDO_USER:-}" ]]; then
        local user_home=$(eval echo ~$SUDO_USER)
        mkdir -p "$user_home/.kube"
        cp /etc/rancher/k3s/k3s.yaml "$user_home/.kube/config"
        chown -R $SUDO_USER:$SUDO_USER "$user_home/.kube"
        log "✓ kubeconfig 已复制到 $user_home/.kube/config"
    fi

    # 验证 kubectl 访问
    if kubectl get nodes &> /dev/null; then
        log "✓ kubectl 访问验证成功"
    else
        error "kubectl 无法访问集群"
    fi
}

# ==================== 验证安装 ====================
verify_installation() {
    log "验证安装结果..."

    echo ""
    log "========== 集群信息 =========="
    kubectl cluster-info

    echo ""
    log "========== 节点状态 =========="
    kubectl get nodes -o wide

    echo ""
    log "========== Namespace 列表 =========="
    kubectl get namespaces

    echo ""
    log "========== ResourceQuota 状态 =========="
    kubectl get resourcequota -A

    echo ""
    log "========== SGX Device Plugin 状态 =========="
    kubectl get daemonset intel-sgx-plugin -n kube-system || warn "SGX Device Plugin 未找到"

    echo ""
    log "========== 节点可分配资源 =========="
    kubectl describe node | grep -A 10 "Allocatable:" || true

    log "✓ 安装验证完成"
}

# ==================== 主流程 ====================
main() {
    log "=========================================="
    log "开始 k3s 集群初始化 (STORY-1.1)"
    log "=========================================="

    check_root
    check_os
    check_sgx_support
    check_resources

    install_k3s
    wait_for_k3s

    create_namespaces
    apply_resource_quotas
    install_sgx_device_plugin
    setup_kubeconfig

    verify_installation

    log ""
    log "=========================================="
    log "✓ k3s 集群初始化完成！"
    log "=========================================="
    log ""
    log "后续步骤:"
    log "  1. 运行 'kubectl get nodes' 验证集群状态"
    log "  2. 部署 cert-manager (STORY-1.2)"
    log "  3. 配置 ArgoCD (STORY-1.3)"
    log ""
    log "文档: /home/ubuntu/service_layer/k8s/platform/README.md"
}

main "$@"
