#!/bin/bash
#
# What it does:
#   1. Downloads fabric-samples 1.3.0
#   2. Fixes docker-compose -> docker compose in scripts
#   3. Fixes capabilities in configtx.yaml
#   4. Copies chaincode to the correct location
#   5. Copies vendor dependencies
#

set -e  # stop on any error

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC}  $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}    $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ─── Configuration ───────────────────────────────────────────────────────────
FABRIC_VERSION="1.3.0"
CA_VERSION="1.3.0"
THIRDPARTY_VERSION="0.4.13"
CHAINCODE_NAME="medical"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FABRIC_SAMPLES_DIR="$SCRIPT_DIR/fabric-samples"
CHAINCODE_SRC="$SCRIPT_DIR/chaincode"
CHAINCODE_DST="$FABRIC_SAMPLES_DIR/chaincode/$CHAINCODE_NAME"
FIRST_NETWORK="$FABRIC_SAMPLES_DIR/first-network"

# Step 0: Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."

    command -v docker >/dev/null 2>&1      || log_error "Docker is not installed"
    docker compose version >/dev/null 2>&1 || log_error "Docker Compose v2 is not installed"
    command -v git >/dev/null 2>&1         || log_error "Git is not installed"
    command -v curl >/dev/null 2>&1        || log_error "curl is not installed"

    docker ps >/dev/null 2>&1 || log_error "Docker daemon is not running. Please start Docker Desktop."

    log_success "All dependencies found"
}

# Step 1: Clone fabric-samples and download binaries/images
download_fabric() {
    log_info "Step 1: Cloning hyperledger/fabric-samples (release-1.4)..."

    if [ -d "$FABRIC_SAMPLES_DIR" ]; then
        log_warn "fabric-samples directory already exists — skipping clone"
    else
        git clone --branch release-1.4 \
            https://github.com/hyperledger/fabric-samples.git \
            "$FABRIC_SAMPLES_DIR"
        log_success "fabric-samples cloned"
    fi

    log_info "Downloading bootstrap.sh..."

    mkdir -p "$FABRIC_SAMPLES_DIR/scripts"

    curl -sS https://raw.githubusercontent.com/hyperledger/fabric/master/scripts/bootstrap.sh \
         -o "$FABRIC_SAMPLES_DIR/scripts/bootstrap.sh"

    chmod +x "$FABRIC_SAMPLES_DIR/scripts/bootstrap.sh"

    log_info "Downloading Fabric $FABRIC_VERSION binaries and Docker images..."

    # Run from SCRIPT_DIR (parent of fabric-samples)
    # bootstrap.sh will cd into fabric-samples and place bin/ there
    cd "$SCRIPT_DIR"
    "$FABRIC_SAMPLES_DIR/scripts/bootstrap.sh" "$FABRIC_VERSION" "$CA_VERSION" "$THIRDPARTY_VERSION"

    log_success "Fabric $FABRIC_VERSION binaries and Docker images downloaded"
}

# Step 2: Fix docker-compose -> docker compose
fix_docker_compose() {
    log_info "Step 2: Replacing docker-compose calls with docker compose..."

    cd "$FIRST_NETWORK"

    # byfn.sh
    sed -i 's/IMAGE_TAG=$IMAGETAG docker-compose/IMAGE_TAG=$IMAGETAG docker compose/g' byfn.sh
    sed -i 's/docker-compose \$COMPOSE_FILES/docker compose $COMPOSE_FILES/g' byfn.sh
    sed -i 's/docker-compose -f/docker compose -f/g' byfn.sh

    # eyfn.sh
    sed -i 's/IMAGE_TAG=\${IMAGETAG} docker-compose/IMAGE_TAG=${IMAGETAG} docker compose/g' eyfn.sh
    sed -i 's/IMAGE_TAG=$IMAGETAG docker-compose/IMAGE_TAG=$IMAGETAG docker compose/g' eyfn.sh
    sed -i 's/docker-compose -f/docker compose -f/g' eyfn.sh

    remaining=$(grep -rn "docker-compose" byfn.sh eyfn.sh | grep -v "echo\|#\|yaml\|file" | wc -l)
    if [ "$remaining" -gt 0 ]; then
        log_warn "Some docker-compose calls remain — please check manually"
    fi

    log_success "docker-compose replaced with docker compose"
}

# Step 3: Fix capabilities in configtx.yaml
fix_capabilities() {
    log_info "Step 3: Fixing capabilities in configtx.yaml..."

    cd "$FIRST_NETWORK"

    # Target state for Fabric 1.3.0:
    #   Channel:     V1_3: true  (others false)
    #   Orderer:     V1_1: true  (others false)
    #   Application: V1_3: true  (others false)

    # Disable all V1_4.x capabilities (not supported by 1.3.0)
    sed -i 's/V1_4_3: true/V1_4_3: false/g' configtx.yaml
    sed -i 's/V1_4_2: true/V1_4_2: false/g' configtx.yaml

    # Channel: enable V1_3, disable V1_1
    # Application: enable V1_3, disable V1_1 and V1_2
    # (V1_3 lines that are false -> true)
    sed -i 's/V1_3: false/V1_3: true/g' configtx.yaml

    # Orderer: V1_4_2 already disabled above, V1_1 should be true
    # (V1_1 lines that are false -> true only under Orderer section)
    # We use python for section-aware replacement to avoid false positives
    python3 - <<'PYEOF'
import re

with open("configtx.yaml", "r") as f:
    content = f.read()

# Set Orderer V1_1: true (it may be false in original)
# Find Orderer capabilities block and set V1_1: true
content = re.sub(
    r'(Orderer: &OrdererCapabilities.*?)(V1_1: false)',
    lambda m: m.group(1) + 'V1_1: true',
    content,
    flags=re.DOTALL
)

with open("configtx.yaml", "w") as f:
    f.write(content)
PYEOF

    log_success "Capabilities fixed (backup saved: configtx.yaml.bak)"
    log_info "  Channel:     V1_3: true"
    log_info "  Orderer:     V1_1: true"
    log_info "  Application: V1_3: true"
}

# Step 4: Copy chaincode
prepare_chaincode() {
    log_info "Step 4: Copying chaincode..."

    if [ ! -f "$CHAINCODE_SRC/contract.go" ] || [ ! -f "$CHAINCODE_SRC/access.go" ]; then
        log_error "Chaincode files not found in $CHAINCODE_SRC. Make sure contract.go and access.go exist."
    fi

    mkdir -p "$CHAINCODE_DST"
    cp "$CHAINCODE_SRC"/*.go "$CHAINCODE_DST/"

    log_success "Chaincode copied to $CHAINCODE_DST"
}

# Step 5: Copy vendor dependencies
copy_vendor() {
    log_info "Step 5: Copying vendor dependencies..."

    ABAC_VENDOR="$FABRIC_SAMPLES_DIR/chaincode/abac/go/vendor"

    if [ ! -d "$ABAC_VENDOR" ]; then
        log_error "Vendor directory not found at $ABAC_VENDOR. Make sure fabric-samples was downloaded."
    fi

    cp -r "$ABAC_VENDOR" "$CHAINCODE_DST/"

    log_success "Vendor dependencies copied"
}

# Final instructions
print_next_steps() {
    echo ""
    echo -e "${GREEN}════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Setup complete!${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo -e "  ${BLUE}1.${NC} Start the network:"
    echo "     cd fabric-samples/first-network"
    echo "     ./byfn.sh up -i 1.3.0"
    echo ""
    echo -e "  ${BLUE}2.${NC} Enter the CLI container:"
    echo "     docker exec -it cli bash"
    echo ""
    echo -e "  ${BLUE}3.${NC} Install the chaincode:"
    echo "     peer chaincode install -n medical -v 1.0 -l golang -p github.com/chaincode/medical/"
    echo ""
    echo -e "  ${BLUE}4.${NC} Instantiate the chaincode:"
    echo "     peer chaincode instantiate \\"
    echo "       -o orderer.example.com:7050 --tls true \\"
    echo "       --cafile \$ORDERER_CA \\"
    echo "       -C mychannel -n medical -l golang -v 1.0 \\"
    echo "       -c '{\"Args\":[\"init\"]}' \\"
    echo "       -P 'OR ('\\''Org1MSP.peer'\\'',' \\'\\''Org2MSP.peer'\\'')'"
    echo ""
    echo "  See README.md for full usage instructions."
    echo ""
}

main() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  Hyperledger Fabric Medical Chaincode Setup${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════${NC}"
    echo ""

    check_dependencies
    download_fabric
    fix_docker_compose
    fix_capabilities
    prepare_chaincode
    copy_vendor
    print_next_steps
}

main "$@"