#!/usr/bin/env bash
set -o pipefail

###############################################################################
# HELP & ARGUMENT PARSING
###############################################################################
usage() {
  cat <<EOF
Usage: ${0##*/} NODE [INTERVAL] [--debug]

Poll drop counters on a specific node every INTERVAL seconds (default 30).

The script automatically detects whether Retina or Cilium is running in the cluster
and configures itself accordingly. If neither is found, the script will exit.

The script always tries to port-forward BOTH:
  • The main agent port   (retina:10093 | cilium:9962)
  • The embedded Hubble port (9965) in the same pod

If the port-forward fails for 9965, Hubble is
ignored and a notice is printed once.

Options:
  --debug    Print commands before executing them

In the metrics diff table:
  • Arrows indicate trend: ↑ increases, ↓ decreases, = no change

Examples:
  $ ${0##*/} aks-node-123
  $ ${0##*/} aks-node-456 10
  $ ${0##*/} aks-node-123 30 --debug
EOF
  exit 1
}

# Show usage if no arguments or help requested
[[ $# -eq 0 || "$1" == "-h" || "$1" == "--help" ]] && usage

# Debug output function (defined early since it's used in detection)
debug() {
  [[ $DEBUG -eq 1 ]] && echo "DEBUG: $*" >&2
}

# Set initial DEBUG flag for early usage
DEBUG=0
for arg in "$@"; do
  [[ "$arg" == "--debug" ]] && DEBUG=1 && break
done

# Function to detect agent type automatically
detect_agent_type() {
  debug "Detecting agent type in cluster..."

  # Check for Retina pods first - try k8s-app=retina (more common)
  local retina_pods=$(kubectl get pods -n kube-system -l k8s-app=retina --no-headers 2>/dev/null | wc -l)
  if [[ $retina_pods -gt 0 ]]; then
    debug "Found $retina_pods Retina pods in cluster (k8s-app=retina)"
    echo "retina"
    return 0
  fi
  
  # Check for Retina pods with alternative label
  local retina_alt_pods=$(kubectl get pods -n kube-system -l app=retina --no-headers 2>/dev/null | wc -l)
  if [[ $retina_alt_pods -gt 0 ]]; then
    debug "Found $retina_alt_pods Retina pods in cluster (app=retina)"
    echo "retina"
    return 0
  fi
  
  # Check for Cilium pods
  local cilium_pods=$(kubectl get pods -n kube-system -l k8s-app=cilium --no-headers 2>/dev/null | wc -l)
  if [[ $cilium_pods -gt 0 ]]; then
    debug "Found $cilium_pods Cilium pods in cluster"
    echo "cilium"
    return 0
  fi
  
  # Check for alternative Cilium selectors
  local cilium_alt_pods=$(kubectl get pods -n kube-system -l app=cilium --no-headers 2>/dev/null | wc -l)
  if [[ $cilium_alt_pods -gt 0 ]]; then
    debug "Found $cilium_alt_pods Cilium pods (with app=cilium selector) in cluster"
    echo "cilium"
    return 0
  fi
  
  # No supported agent found
  echo ""
  return 1
}

# Detect agent type automatically
echo "Detecting network monitoring agent in cluster..."
AGENT_TYPE=$(detect_agent_type)

if [[ -z "$AGENT_TYPE" ]]; then
  echo "Error: No supported network monitoring agent found in cluster"
  echo "This script supports:"
  echo "  • Retina (pods with label app=retina)"
  echo "  • Cilium (pods with label k8s-app=cilium or app=cilium)"
  echo ""
  echo "Available pods in kube-system namespace:"
  kubectl get pods -n kube-system --no-headers | head -20
  exit 1
fi

echo "Detected agent type: $AGENT_TYPE"

# Configure agent-specific settings
if [[ "$AGENT_TYPE" == "retina" ]]; then
  AGENT_PORT=10093
  AGENT_NS="kube-system"
  AGENT_SELECTOR="k8s-app=retina"
elif [[ "$AGENT_TYPE" == "cilium" ]]; then
  AGENT_PORT=9962
  AGENT_NS="kube-system"
  # Use the more common selector first, fallback handled in get_agent_pod function
  AGENT_SELECTOR="k8s-app=cilium"
else
  echo "Error: Unsupported agent type detected: $AGENT_TYPE"
  exit 1
fi

# Parse node name (now the first argument)
if [[ $# -eq 0 ]]; then
  echo "Error: NODE name is required"
  usage
fi
NODE="$1"
shift

# Parse interval (optional)
INTERVAL=30
if [[ $# -gt 0 && "$1" != "--debug" ]]; then
  if [[ "$1" =~ ^[0-9]+$ ]]; then
    INTERVAL="$1"
    shift
  else
    echo "Error: INTERVAL must be a number"
    usage
  fi
fi

# Parse debug flag (optional)
DEBUG=0
if [[ $# -gt 0 && "$1" == "--debug" ]]; then
  DEBUG=1
fi

###############################################################################
# FUNCTIONS
###############################################################################

# Colorize text if supported
colorize() {
  local text=$1
  local type=$2  # "good", "bad", or "neutral"
  
  # Check if we're in a terminal that supports colors
  if [[ -t 1 && -n "$TERM" && "$TERM" != "dumb" ]]; then
    if [[ "$type" == "good" ]]; then
      echo -e "\033[32m$text\033[0m"  # Green
    elif [[ "$type" == "bad" ]]; then
      echo -e "\033[31m$text\033[0m"  # Red
    elif [[ "$type" == "neutral" ]]; then
      echo -e "\033[33m$text\033[0m"  # Yellow
    else
      echo "$text"
    fi
  else
    echo "$text"
  fi
}

# Get the pod name for the agent running on the specified node
get_agent_pod() {
  local node=$1
  
  debug "kubectl get pods -n $AGENT_NS -l $AGENT_SELECTOR --field-selector spec.nodeName=$node -o jsonpath='{.items[0].metadata.name}'"
  
  # First check if any pods match our criteria and output more detail if we have issues
  local pod_count=$(kubectl get pods -n $AGENT_NS -l $AGENT_SELECTOR --field-selector spec.nodeName=$node -o name 2>/dev/null | wc -l)
  
  # If no pods found with primary selector, try alternative selectors
  if [[ $pod_count -eq 0 && "$AGENT_TYPE" == "cilium" && "$AGENT_SELECTOR" == "k8s-app=cilium" ]]; then
    debug "No pods found with k8s-app=cilium selector, trying app=cilium..."
    AGENT_SELECTOR="app=cilium"
    pod_count=$(kubectl get pods -n $AGENT_NS -l $AGENT_SELECTOR --field-selector spec.nodeName=$node -o name 2>/dev/null | wc -l)
  elif [[ $pod_count -eq 0 && "$AGENT_TYPE" == "retina" && "$AGENT_SELECTOR" == "k8s-app=retina" ]]; then
    debug "No pods found with k8s-app=retina selector, trying app=retina..."
    AGENT_SELECTOR="app=retina"
    pod_count=$(kubectl get pods -n $AGENT_NS -l $AGENT_SELECTOR --field-selector spec.nodeName=$node -o name 2>/dev/null | wc -l)
  fi
  
  if [[ $pod_count -eq 0 ]]; then
    echo "No pods found with selector '$AGENT_SELECTOR' on node '$node'" >&2
    echo "Available pods in namespace $AGENT_NS:" >&2
    kubectl get pods -n $AGENT_NS -o wide | grep $node >&2 || true
    echo "Trying to find any retina or cilium pods on this node:" >&2
    kubectl get pods -n $AGENT_NS -o wide | grep -E "(retina|cilium)" | grep $node >&2 || true
    return 1
  fi
  
  kubectl get pods -n $AGENT_NS -l $AGENT_SELECTOR --field-selector spec.nodeName=$node -o jsonpath='{.items[0].metadata.name}'
}

# Setup port forwarding
setup_port_forward() {
  local pod=$1
  local local_port=$2
  local remote_port=$3
  local pid_var=$4
  
  debug "kubectl port-forward -n $AGENT_NS $pod $local_port:$remote_port &"
  
  # Check if the port is already in use
  if lsof -i:$local_port &>/dev/null; then
    echo "Port $local_port is already in use. Killing process..."
    lsof -i:$local_port -t | xargs kill -9 2>/dev/null || true
    sleep 1
  fi
  
  # Start port forwarding
  kubectl port-forward -n $AGENT_NS "$pod" "$local_port:$remote_port" &
  eval "$pid_var=$!"
  
  # Give port-forward a moment to establish
  sleep 2
  
  # Check if port-forward is still running
  if ! kill -0 "${!pid_var}" 2>/dev/null; then
    echo "Port-forward process ${!pid_var} failed to start or died immediately"
    return 1
  fi
  
  # Verify the port is actually listening
  if ! lsof -i:$local_port &>/dev/null; then
    echo "Port $local_port is not listening after port-forward setup"
    return 1
  fi
  
  return 0
}

# Get metrics from endpoint
get_metrics() {
  local endpoint=$1
  
  debug "curl -s -m 5 $endpoint"
  local metrics
  metrics=$(curl -s -m 5 "$endpoint" || echo "")
  
  if [[ $DEBUG -eq 1 && -n "$metrics" ]]; then
    local filename="$(date +%Y%m%d-%H%M%S)-${endpoint##*/}-metrics.txt"
    filename="${filename//\//_}"
    echo "$metrics" > "$filename"
    debug "Metrics saved to $filename"
  fi
  
  echo "$metrics"
}

# Extract drop counts from metrics
parse_drops() {
  local metrics=$1
  local prefix=$2
  local alternative_patterns=${3:-""}  # Optional additional patterns to search with default empty string
  
  if [[ -z "$metrics" ]]; then
    return 0
  fi
  
  local drop_metrics=""
  
  # First try specific patterns based on the prefix
  if [[ "$prefix" == "cilium" ]]; then
    # For cilium, check multiple drop-related patterns
    drop_metrics=$(echo "$metrics" | grep -E "(drop_count_total|drop_bytes_total|_dropped|_errors_total)" | sort)
  elif [[ "$prefix" == "retina" ]]; then
    # For retina, use its specific patterns - Retina uses different metric names
    drop_metrics=$(echo "$metrics" | grep -E "(drop_count|dropped_packets|packet_drop|networkobservability.*drop)" | sort)
  elif [[ "$prefix" == "hubble" ]]; then
    # For hubble, use its specific patterns
    drop_metrics=$(echo "$metrics" | grep -E "(hubble_drop_total|hubble.*drop|flow.*drop)" | sort)
  else
    # Generic fallback
    drop_metrics=$(echo "$metrics" | grep -E "(drop|dropped)" | sort)
  fi
  
  # If no metrics found with specific patterns, try generic drop patterns
  if [[ -z "$drop_metrics" ]]; then
    debug "No metrics found with $prefix patterns, trying generic drop patterns..."
    drop_metrics=$(echo "$metrics" | grep -iE "(drop|dropped)" | grep -v " 0$" | sort)
  fi
  
  # If alternative patterns were provided and no metrics found, try those
  if [[ -z "$drop_metrics" && -n "$alternative_patterns" ]]; then
    drop_metrics=$(echo "$metrics" | grep -E "$alternative_patterns" | sort)
  fi
  
  if [[ -z "$drop_metrics" && $DEBUG -eq 1 ]]; then
    echo "DEBUG: No drop metrics found with standard patterns for prefix '$prefix'" >&2
    echo "DEBUG: Trying generic 'drop' pattern..." >&2
    local generic_drops=$(echo "$metrics" | grep -i "drop" | head -5)
    if [[ -n "$generic_drops" ]]; then
      echo "DEBUG: Found some metrics containing 'drop':" >&2
      echo "$generic_drops" >&2
    else
      echo "DEBUG: No metrics containing 'drop' found" >&2
      echo "DEBUG: Looking for error or packet metrics..." >&2
      echo "$metrics" | grep -i -E "(error|packet|reject)" | head -5 >&2
    fi
    
    # Create a debug file with all metrics for analysis
    local debug_file="${prefix}_all_metrics.txt"
    echo "$metrics" > "$debug_file"
    echo "DEBUG: All metrics saved to $debug_file for analysis" >&2
  fi
  
  echo "$drop_metrics"
}

# Cleanup function
cleanup() {
  debug "Cleaning up port forwards"
  [[ -n "${AGENT_PF_PID:-}" ]] && kill -9 $AGENT_PF_PID 2>/dev/null || true
  [[ -n "${HUBBLE_PF_PID:-}" ]] && kill -9 $HUBBLE_PF_PID 2>/dev/null || true
  exit 0
}

# Extract drop counts into a structured format
extract_structured_drops() {
  local metrics=$1
  local agent_type=$2
  local output_file=$3
  
  debug "Extracting structured drops for $agent_type to $output_file"
  debug "Input metrics has $(echo "$metrics" | wc -l) lines"
  
  local line_count=0
  # Process each line of the metrics to a simpler format
  while IFS= read -r line; do
    # Skip comment and empty lines
    [[ "$line" =~ ^#.*$ || -z "$line" ]] && continue
    
    # Skip zero values if not in debug mode
    [[ "$line" =~ [[:space:]]0$ && $DEBUG -eq 0 ]] && continue
    
    # Extract the key information and append to the output file
    echo "$agent_type: $line" >> "$output_file"
    line_count=$((line_count + 1))
    [[ $DEBUG -eq 1 ]] && debug "Added line: $agent_type: $line"
  done <<< "$metrics"
  
  # Debug check file contents
  debug "Wrote $line_count lines to $output_file"
  if [[ $DEBUG -eq 1 && -f "$output_file" ]]; then
    debug "File contents (first 10 lines):"
    head -10 "$output_file" >&2
  fi
}

# Create a comparison table from old and new metrics
generate_diff_table() {
  local old_metrics_file=$1
  local new_metrics_file=$2
  local output_file=$3
  
  debug "Generating diff table: comparing $old_metrics_file and $new_metrics_file"
  
  # Ensure output file is empty
  > "$output_file"
  
  # Check if files exist and have content
  if [[ ! -s "$old_metrics_file" ]]; then
    debug "Warning: Old metrics file is empty or doesn't exist"
    echo "No previous metrics data available for comparison" > "$output_file"
    return 0
  fi
  
  if [[ ! -s "$new_metrics_file" ]]; then
    debug "Warning: New metrics file is empty or doesn't exist"
    echo "No current metrics data available for comparison" > "$output_file"
    return 0
  fi
  
  # Create table header
  echo "=== Drop Counter Changes ===" > "$output_file"
  echo "" >> "$output_file"
  printf "%-80s %15s %15s %15s\n" "Metric Name" "Previous" "Current" "Change" >> "$output_file"
  printf "%-80s %15s %15s %15s\n" "$(printf '%*s' 80 '' | tr ' ' '-')" "$(printf '%*s' 15 '' | tr ' ' '-')" "$(printf '%*s' 15 '' | tr ' ' '-')" "$(printf '%*s' 15 '' | tr ' ' '-')" >> "$output_file"
  
  # Create associative arrays to store old and new values
  declare -A old_values new_values
  local changes_found=0
  
  # Parse old metrics
  while IFS= read -r line; do
    if [[ -n "$line" && ! "$line" =~ ^# ]]; then
      # Extract metric name and value
      local metric_name=$(echo "$line" | sed 's/^[^:]*: //' | awk '{print $1}')
      local metric_value=$(echo "$line" | awk '{print $NF}')
      old_values["$metric_name"]="$metric_value"
    fi
  done < "$old_metrics_file"
  
  # Parse new metrics
  while IFS= read -r line; do
    if [[ -n "$line" && ! "$line" =~ ^# ]]; then
      # Extract metric name and value
      local metric_name=$(echo "$line" | sed 's/^[^:]*: //' | awk '{print $1}')
      local metric_value=$(echo "$line" | awk '{print $NF}')
      new_values["$metric_name"]="$metric_value"
    fi
  done < "$new_metrics_file"
  
  # Compare metrics and generate table rows
  local all_metrics=()
  
  # Collect all unique metric names
  for metric in "${!old_values[@]}"; do
    all_metrics+=("$metric")
  done
  for metric in "${!new_values[@]}"; do
    if [[ ! " ${all_metrics[*]} " =~ " ${metric} " ]]; then
      all_metrics+=("$metric")
    fi
  done
  
  # Sort metrics for consistent output
  IFS=$'\n' all_metrics=($(sort <<<"${all_metrics[*]}"))
  unset IFS
  
  # Generate table rows
  for metric in "${all_metrics[@]}"; do
    local old_val="${old_values[$metric]:-0}"
    local new_val="${new_values[$metric]:-0}"
    
    # Only show metrics that have changed or are non-zero
    if [[ "$old_val" != "$new_val" ]] || [[ "$old_val" != "0" ]] || [[ "$new_val" != "0" ]]; then
      local change=""
      local change_indicator=""
      
      # Calculate numeric change if both values are numbers (including scientific notation)
      if [[ "$old_val" =~ ^[0-9]+\.?[0-9]*([eE][+-]?[0-9]+)?$ ]] && [[ "$new_val" =~ ^[0-9]+\.?[0-9]*([eE][+-]?[0-9]+)?$ ]]; then
        # Use awk for floating point arithmetic to handle scientific notation
        local diff=$(awk "BEGIN {printf \"%.0f\", $new_val - $old_val}")
        if [[ $diff -gt 0 ]]; then
          change="+$diff"
          change_indicator="↑"
        elif [[ $diff -lt 0 ]]; then
          change="$diff"
          change_indicator="↓"
        else
          change="0"
          change_indicator="="
        fi
      else
        # Non-numeric comparison
        if [[ "$old_val" != "$new_val" ]]; then
          change="changed"
          change_indicator="~"
        else
          change="same"
          change_indicator="="
        fi
      fi
      
      # Don't truncate metric names - show full names
      local display_metric="$metric"
      
      # Format change with arrow indicators (no colors)
      local formatted_change="$change $change_indicator"
      
      printf "%-80s %15s %15s %15s\n" "$display_metric" "$old_val" "$new_val" "$formatted_change" >> "$output_file"
      changes_found=1
    fi
  done
  
  if [[ $changes_found -eq 0 ]]; then
    echo "" >> "$output_file"
    echo "No changes detected in drop counters" >> "$output_file"
  else
    echo "" >> "$output_file"
    echo "Legend: ↑ = increase (potential issue), ↓ = decrease (improvement), = = no change" >> "$output_file"
  fi
  
  # Add note about detailed metrics file
  echo "" >> "$output_file"
  echo "Note: For full metric details and complete names, see: $new_metrics_file" >> "$output_file"
  
  return $changes_found
}

###############################################################################
# MAIN SCRIPT
###############################################################################

# Check for required commands
for cmd in kubectl curl grep sort; do
  if ! command -v $cmd &>/dev/null; then
    echo "Error: Required command '$cmd' not found"
    exit 1
  fi
done

# Optional command check
for cmd in lsof; do
  if ! command -v $cmd &>/dev/null; then
    echo "Warning: Optional command '$cmd' not found, some features may be limited"
  fi
done

# Verify kubectl can connect to the cluster
if ! kubectl get nodes -o name --request-timeout=5s &>/dev/null; then
  echo "Error: Cannot connect to Kubernetes cluster. Please check your kubeconfig."
  exit 1
fi

# Verify the node exists in the cluster
if ! kubectl get nodes -o name --request-timeout=5s | grep -q "${NODE}"; then
  echo "Error: Node '${NODE}' not found in the cluster."
  echo "Available nodes:"
  kubectl get nodes
  exit 1
fi

# Setup trap for cleanup
trap cleanup EXIT INT TERM

echo "Watching drop counters on node $NODE (${AGENT_TYPE}) every ${INTERVAL}s..."

# Local ports for port forwarding
LOCAL_AGENT_PORT=8093
LOCAL_HUBBLE_PORT=8965
HUBBLE_PORT=9965
HUBBLE_ENABLED=0
HUBBLE_WARNING_SHOWN=0

# Files to store metrics for comparison
METRICS_DIR="${TMPDIR:-/tmp}/watchmetrics"
mkdir -p "$METRICS_DIR"
CURRENT_METRICS_FILE="$METRICS_DIR/current_metrics.txt"
PREVIOUS_METRICS_FILE="$METRICS_DIR/previous_metrics.txt"
DIFF_TABLE_FILE="$METRICS_DIR/diff_table.txt"

# Initialize files if they don't exist
> "$CURRENT_METRICS_FILE"
> "$PREVIOUS_METRICS_FILE"
> "$DIFF_TABLE_FILE"

debug "Metrics files:"
debug "- Current: $CURRENT_METRICS_FILE"
debug "- Previous: $PREVIOUS_METRICS_FILE" 
debug "- Diff Table: $DIFF_TABLE_FILE"

while true; do
  echo -e "\n$(date): Checking ${AGENT_TYPE} drop counters on $NODE"
  
  # Clear current metrics for this iteration
  > "$CURRENT_METRICS_FILE"
  
  # Get the agent pod on the specified node
  echo "Looking for ${AGENT_TYPE} pod on node $NODE..."
  AGENT_POD=$(get_agent_pod "$NODE")
  
  if [[ -z "$AGENT_POD" ]]; then
    echo "Error: No ${AGENT_TYPE} pod found on node $NODE"
    echo "Checking node existence..."
    kubectl get nodes | grep "$NODE" || echo "Node $NODE doesn't exist in the cluster"
    echo "Waiting ${INTERVAL}s before next check..."
    sleep $INTERVAL
    continue
  fi
  
  echo "Found pod: $AGENT_POD"
  
  # Kill any existing port-forwards
  [[ -n "${AGENT_PF_PID:-}" ]] && kill -9 $AGENT_PF_PID 2>/dev/null || true
  [[ -n "${HUBBLE_PF_PID:-}" ]] && kill -9 $HUBBLE_PF_PID 2>/dev/null || true
  
  # Counter for number of iterations with no metrics found
  if [[ -z "${NO_METRICS_COUNT:-}" ]]; then
    NO_METRICS_COUNT=0
  fi
  
  # Flag to track if any metrics were found in this iteration
  FOUND_ANY_METRICS=0
  
  # Setup port forwarding for agent metrics
  echo "Setting up port forwarding for agent metrics to port $AGENT_PORT..."
  if setup_port_forward "$AGENT_POD" "$LOCAL_AGENT_PORT" "$AGENT_PORT" "AGENT_PF_PID"; then
    echo "Port forwarding established for agent metrics"
    
    # Get and display agent drop metrics
    echo "Fetching metrics from http://localhost:$LOCAL_AGENT_PORT/metrics..."
    AGENT_METRICS=$(get_metrics "http://localhost:$LOCAL_AGENT_PORT/metrics")
    
    if [[ -z "$AGENT_METRICS" ]]; then
      echo "Warning: No metrics data received from agent endpoint"
    fi
    
    # Try with the standard pattern first
    AGENT_DROPS=$(parse_drops "$AGENT_METRICS" "$AGENT_TYPE")
    
    if [[ -n "$AGENT_DROPS" ]]; then
      echo -e "\n=== Agent Drop Counters ==="
      echo "$AGENT_DROPS"
      FOUND_ANY_METRICS=1
      
      # Extract structured metrics for comparison
      extract_structured_drops "$AGENT_DROPS" "$AGENT_TYPE" "$CURRENT_METRICS_FILE"
    else
      echo "No drop counters found in agent metrics"
      
      # Try again with a fallback pattern focused on errors
      FALLBACK_DROPS=$(echo "$AGENT_METRICS" | grep -E "(^${AGENT_TYPE}.*error|^${AGENT_TYPE}.*fail)" | grep -v "0$" | sort)
      if [[ -n "$FALLBACK_DROPS" ]]; then
        echo -e "\n=== Agent Error Counters (potential drops) ==="
        echo "$FALLBACK_DROPS"
        FOUND_ANY_METRICS=1
        
        # Extract structured metrics for comparison
        extract_structured_drops "$FALLBACK_DROPS" "$AGENT_TYPE" "$CURRENT_METRICS_FILE"
      elif [[ $DEBUG -eq 1 ]]; then
        echo "Sample of received metrics (first 10 lines):"
        echo "$AGENT_METRICS" | head -10
      fi
    fi
  else
    echo "Failed to establish port forwarding for agent metrics"
  fi
  
  # Setup port forwarding for Hubble metrics if not disabled
  if [[ $HUBBLE_ENABLED -eq 0 || $HUBBLE_WARNING_SHOWN -eq 0 ]]; then
    echo "Setting up port forwarding for Hubble metrics to port $HUBBLE_PORT..."
    if setup_port_forward "$AGENT_POD" "$LOCAL_HUBBLE_PORT" "$HUBBLE_PORT" "HUBBLE_PF_PID"; then
      HUBBLE_ENABLED=1
      
      # Get and display Hubble drop metrics
      echo "Fetching metrics from http://localhost:$LOCAL_HUBBLE_PORT/metrics..."
      HUBBLE_METRICS=$(get_metrics "http://localhost:$LOCAL_HUBBLE_PORT/metrics")
      
      if [[ -z "$HUBBLE_METRICS" ]]; then
        echo "Warning: No metrics data received from Hubble endpoint"
      else
        # Try with hubble prefix first, then fall back to flow metrics if needed
        HUBBLE_DROPS=$(parse_drops "$HUBBLE_METRICS" "hubble" "flow.*drop|flow.*reject")
        
        if [[ -n "$HUBBLE_DROPS" ]]; then
          echo -e "\n=== Hubble Drop Counters ==="
          echo "$HUBBLE_DROPS"
          FOUND_ANY_METRICS=1
          
          # Extract structured metrics for comparison
          extract_structured_drops "$HUBBLE_DROPS" "hubble" "$CURRENT_METRICS_FILE"
        else
          echo "No drop counters found in Hubble metrics"
          
          if [[ $DEBUG -eq 1 ]]; then
            echo "Sample of received Hubble metrics (first 10 lines):"
            echo "$HUBBLE_METRICS" | head -10
            
            # Try to find flow-related metrics as they might be useful
            echo "Looking for flow metrics (might contain useful information):"
            FLOW_METRICS=$(echo "$HUBBLE_METRICS" | grep -i "flow" | head -10)
            if [[ -n "$FLOW_METRICS" ]]; then
              echo "$FLOW_METRICS"
            else
              echo "No flow metrics found"
            fi
          fi
        fi
      fi
    else
      HUBBLE_ENABLED=0
      if [[ $HUBBLE_WARNING_SHOWN -eq 0 ]]; then
        echo "Notice: Hubble port-forwarding failed - Hubble metrics will be ignored"
        HUBBLE_WARNING_SHOWN=1
        
        if [[ $DEBUG -eq 1 ]]; then
          echo "DEBUG: Checking if Hubble is enabled in Cilium..."
          kubectl -n $AGENT_NS exec $AGENT_POD -- cilium status | grep -i hubble || echo "Hubble not found in Cilium status"
        fi
      fi
    fi
  fi
  
  # Update metrics counter
  if [[ $FOUND_ANY_METRICS -eq 0 ]]; then
    NO_METRICS_COUNT=$((NO_METRICS_COUNT + 1))
    
    # After 3 attempts with no metrics, provide some help
    if [[ $NO_METRICS_COUNT -eq 3 ]]; then
      echo -e "\n=================================================================================================="
      echo "NOTE: No drop metrics found after multiple attempts. This could be due to:"
      echo "  1. There are genuinely no packet drops occurring on this node"
      echo "  2. The metrics format used by ${AGENT_TYPE} in your cluster is different than expected"
      echo "  3. ${AGENT_TYPE} is not configured to expose drop metrics"
      echo ""
      if [[ "$AGENT_TYPE" == "cilium" ]]; then
        echo "For Cilium, you might want to check:"
        echo "  - Cilium version: kubectl -n kube-system exec $AGENT_POD -- cilium version"
        echo "  - Hubble status: kubectl -n kube-system exec $AGENT_POD -- cilium status | grep Hubble"
        echo "  - Metrics directly: kubectl -n kube-system port-forward $AGENT_POD 9962:9962 & curl localhost:9962/metrics | grep -i drop"
      elif [[ "$AGENT_TYPE" == "retina" ]]; then
        echo "For Retina, you might want to check:"
        echo "  - Retina version: kubectl -n kube-system describe pod $AGENT_POD | grep Image"
        echo "  - Metrics directly: kubectl -n kube-system port-forward $AGENT_POD 10093:10093 & curl localhost:10093/metrics | grep -i drop"
      fi
      echo "=================================================================================================="
      
      # Reset counter so we don't show this message too often
      NO_METRICS_COUNT=0
    fi
  else
    # Reset the counter when metrics are found
    NO_METRICS_COUNT=0
    
    # Generate and display the diff table if we have previous metrics
    if [[ -s "$PREVIOUS_METRICS_FILE" && -s "$CURRENT_METRICS_FILE" ]]; then
      echo -e "\n=== Metrics Changes Since Last Check ==="
      debug "Comparing metrics in $PREVIOUS_METRICS_FILE and $CURRENT_METRICS_FILE"
      
      # Generate the diff output
      generate_diff_table "$PREVIOUS_METRICS_FILE" "$CURRENT_METRICS_FILE" "$DIFF_TABLE_FILE"
      
      # Display the diff output
      if [[ -s "$DIFF_TABLE_FILE" ]]; then
        cat "$DIFF_TABLE_FILE"
      else
        echo "  No changes in drop counters since last check"
      fi
    else
      debug "Not generating diff - missing or empty metrics files"
    fi
    
    # Save current metrics as previous for next comparison
    if [[ -s "$CURRENT_METRICS_FILE" ]]; then
      debug "Saving current metrics for next comparison"
      cp "$CURRENT_METRICS_FILE" "$PREVIOUS_METRICS_FILE"
    else
      debug "Current metrics file is empty, not saving for comparison"
    fi
    
    # Clear current metrics file for next iteration
    > "$CURRENT_METRICS_FILE"
  fi
  
  echo -e "\nWaiting ${INTERVAL}s before next check..."
  sleep $INTERVAL
done
