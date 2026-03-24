package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

const (
	// DoctorPublicKey database key prefix for doctor keys
	DoctorPublicKey = "__access_public_key_"
	// DoctorAccessKey database key prefix for doctor access
	DoctorAccessKey = "__access_doctor_"
)

// MedicalRecordChaincode is the object that contains all of the chaincode that can be executed
type AccessControl struct {
	record *MedicalRecordChaincode
}

type AccessType uint64

const (
	None AccessType = 0
	Info AccessType = 1
	Full AccessType = 2
)

// registerDoctor adds doctor key to database
func (t *AccessControl) registerDoctor(stub shim.ChaincodeStubInterface, doctor string, doctorKey []byte) (bool, error) {
	key, _ := stub.CreateCompositeKey(DoctorPublicKey, []string{doctor})
	data, err := stub.GetState(key)
	if err != nil {
		return false, err
	}
	if data != nil {
		return false, errors.New("Already registered")
	}
	stub.PutState(key, doctorKey)
	return true, nil
}

// setAccess sets doctor access level, returns true if access was changed
func (t *AccessControl) setAccess(stub shim.ChaincodeStubInterface, patientId uint64, doctor string, accType uint64) (bool, error) {
	accesskey, _ := stub.CreateCompositeKey(DoctorAccessKey, []string{doctor, fmt.Sprint(patientId)})
	current, err := t.record.getValue(stub, accesskey)
	if err != nil || current == accType {
		return false, err
	}
	t.record.setValue(stub, accesskey, accType)
	return true, nil
}

// checkAccess returns doctor access level
func (t *AccessControl) checkAccess(stub shim.ChaincodeStubInterface, patientId uint64, doctor string, caller []byte) (AccessType, error) {
	key, _ := stub.CreateCompositeKey(DoctorPublicKey, []string{doctor})
	data, err := stub.GetState(key)
	if err != nil {
		return 0, err
	}
	if data == nil {
		return 0, errors.New("Doctor not registered")
	}
	if !bytes.Equal(data, caller) {
		return 0, errors.New("Invalid caller certificate")
	}
	accesskey, _ := stub.CreateCompositeKey(DoctorAccessKey, []string{doctor, fmt.Sprint(patientId)})
	current, err := t.record.getValue(stub, accesskey)
	if err != nil {
		return 0, err
	}
	return AccessType(current), nil
}
