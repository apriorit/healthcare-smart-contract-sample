# Hyperledger Fabric - Medical Chaincode Sample

A sample built on Hyperledger Fabric 1.3.0. implements patient data storage and doctor access control using X.509 certificates. 

## Prerequisites

- Docker (running)
- Docker Compose v2
- Go 1.19+
- Git
- Python 3

## Project Structure

```
.
├── README.md
├── setup.sh                        ← automated setup script
└── chaincode/
    ├── contract.go                 ← main chaincode
    └── access.go                   ← access control
```

## Quick Start

```bash
# 1. Run the setup script
chmod +x setup.sh
./setup.sh

# 2. Start the network
cd fabric-samples/first-network
./byfn.sh up -i 1.3.0

# 3. Enter the CLI container
docker exec -it cli bash

# 4. Install the chaincode
peer chaincode install -n medical -v 1.0 -l golang -p github.com/chaincode/medical/

# 5. Instantiate the chaincode
peer chaincode instantiate \
  -o orderer.example.com:7050 --tls true \
  --cafile /opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem \
  -C mychannel -n medical -l golang -v 1.0 \
  -c '{"Args":["init"]}' \
  -P 'OR ('\''Org1MSP.peer'\'','\''Org2MSP.peer'\'')'
```

## Usage
 
All commands are executed inside the CLI container (`docker exec -it cli bash`).
 
First, define these variables once - required for all invoke commands:
 
```bash
ORDERER_CA=/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem
 
ORDERER_FLAGS="-o orderer.example.com:7050 --tls true --cafile $ORDERER_CA"
```

### Register a Patient

```bash
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"RegisterPatient","Args":["{\"firstname\":\"Ivan\",\"lastname\":\"Petrov\",\"gender\":1,\"birthdate\":19900101,\"phone\":\"0501234567\"}"]}'
```

### Register a Doctor

```bash
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"RegisterDoctor","Args":["{\"doctor\":\"dr.house\"}"]}'
```

### Grant Doctor Access

```bash
# access: 1 = Info (patient profile only), 2 = Full (profile + visit history)
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"SetDoctorAccess","Args":["{\"patient\":1,\"doctor\":\"dr.house\",\"access\":2}"]}'
```

### Create a Visit

```bash
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"PatientVisit","Args":["{\"patient\":1,\"doctor\":\"dr.house\",\"complaint\":\"headache\"}"]}'
```

### Set Diagnosis

```bash
# ID — visit number, patient — patient ID
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"SetDiagnosis","Args":["{\"ID\":1,\"patient\":1,\"diagnosis\":\"migraine\"}"]}'
```

### Set Prescription

```bash
peer chaincode invoke $ORDERER_FLAGS -C mychannel -n medical \
  -c '{"function":"SetPerscription","Args":["{\"ID\":1,\"patient\":1,\"perscription\":\"aspirin 500mg\"}"]}'
```

### Get Patient Profile

```bash
peer chaincode query -C mychannel -n medical \
  -c '{"function":"GetPatient","Args":["{\"ID\":1,\"doctor\":\"dr.house\"}"]}'
```

### Get Full Medical Records

```bash
peer chaincode query -C mychannel -n medical \
  -c '{"function":"GetMedicalRecords","Args":["{\"ID\":1,\"doctor\":\"dr.house\"}"]}'
```

## Stopping the Network

```bash
exit  # exit the container
cd fabric-samples/first-network
./byfn.sh down
```
