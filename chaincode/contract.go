package main

import (
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/msp"
	"github.com/hyperledger/fabric/protos/peer"
)

const (
	// PatientInfoKey database key prefix for patient information
	MetadataKey = "__metadata_"
	// PatientInfoKey database key prefix for patient information
	PatientInfoKey = "__patient_record_"
	// PatientInfoCountKey database key prefix for patient count
	PatientInfoCountKey = "__patient_count_"
	// MedVisitKey database key prefix for medical visits
	MedVisitKey = "__visit_record_"
	// VisitInfoCountKey database key prefix for visit count
	VisitInfoCountKey = "__visit_count_"
)

// main function starts up the chaincode in the container during instantiation
func main() {
	if err := shim.Start(new(MedicalRecordChaincode)); err != nil {
		fmt.Printf("Error starting chaincode: %s", err)
	}
}

// MedicalRecordChaincode is the object that contains all of the chaincode that can be executed
type MedicalRecordChaincode struct{}

type PatientInfo struct {
	PatientID uint64 `json:"ID"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Gender    uint64 `json:"gender"` // 0 - Unspecified; 1 - Male; 2 - Female;
	BirthDate uint64 `json:"birthdate"`
	Phone     string `json:"phone"`
}

type MedicalVisit struct {
	VisitID      uint64 `json:"ID"`
	PatientID    uint64 `json:"patient"`
	Doctor       string `json:"doctor"`
	Complaint    string `json:"complaint"`
	Diagnosis    string `json:"diagnosis"`
	Perscription string `json:"perscription"`
}

type PatientInfoParams struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Gender    uint64 `json:"gender"`
	BirthDate uint64 `json:"birthdate"`
	Phone     string `json:"phone"`
}

type PatientIDParams struct {
	PatientID uint64 `json:"ID"`
	Doctor    string `json:"doctor"`
}

type VisitInfoParams struct {
	PatientID uint64 `json:"patient"`
	Doctor    string `json:"doctor"`
	Complaint string `json:"complaint"`
}

type DiagnosisParams struct {
	VisitID   uint64 `json:"ID"`
	PatientID uint64 `json:"patient"`
	Diagnosis string `json:"diagnosis"`
}

type PerscriptionParams struct {
	VisitID      uint64 `json:"ID"`
	PatientID    uint64 `json:"patient"`
	Perscription string `json:"perscription"`
}

type DoctorParams struct {
	Doctor string `json:"doctor"`
}

type SetDoctorAccessParams struct {
	PatientID uint64 `json:"patient"`
	Doctor    string `json:"doctor"`
	Access    uint64 `json:"access"`
}

type MedicalRecords struct {
	Patient PatientInfo    `json:"patient"`
	History []MedicalVisit `json:"history"`
}

// Init runs initialization for chaincode
func (t *MedicalRecordChaincode) Init(stub shim.ChaincodeStubInterface) peer.Response {
	caller, err := callerCN(stub)
	if err != nil {
		return shim.Error("Error getting caller cn")
	}
	ownerKey, err := getOwnerKey(stub)
	if err != nil {
		return shim.Error("Error getting database key")
	}
	err = stub.PutState(ownerKey, []byte(caller))
	if err != nil {
		return shim.Error("Error saving data")
	}
	return shim.Success(nil)
}

// Invoke runs functions of chaincode
func (t *MedicalRecordChaincode) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	fn, args := stub.GetFunctionAndParameters()
	switch fn {
	case "RegisterPatient":
		return t.registerPatient(stub, args)
	case "UpdatePatientDetails":
		return t.updatePatient(stub, args)
	case "GetPatient":
		return t.getPatientById(stub, args)
	case "PatientVisit":
		return t.patientVisit(stub, args)
	case "SetDiagnosis":
		return t.setDiagnosis(stub, args)
	case "SetPerscription":
		return t.setPerscription(stub, args)
	case "GetMedicalRecords":
		return t.getMedicalRecords(stub, args)
	case "RegisterDoctor":
		return t.registerDoctor(stub, args)
	case "SetDoctorAccess":
		return t.setDocAccess(stub, args)
	}
	return shim.Error("Undefined function")
}

func (t *MedicalRecordChaincode) registerPatient(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := PatientInfoParams{}
	_, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	count, _ := t.getValue(stub, PatientInfoCountKey)
	newPatient := PatientInfo{
		PatientID: count + 1,
		FirstName: params.FirstName,
		LastName:  params.LastName,
		Gender:    params.Gender,
		BirthDate: params.BirthDate,
		Phone:     params.Phone,
	}
	t.setPatient(stub, newPatient)
	t.setValue(stub, PatientInfoCountKey, count+1)
	result, _ := json.Marshal(newPatient)
	return shim.Success(result)
}

func (t *MedicalRecordChaincode) setDocAccess(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := SetDoctorAccessParams{}
	_, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	ac := AccessControl{t}
	_, err = ac.setAccess(stub, params.PatientID, params.Doctor, params.Access)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (t *MedicalRecordChaincode) registerDoctor(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := DoctorParams{}
	caller, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	ac := AccessControl{t}
	_, err = ac.registerDoctor(stub, params.Doctor, []byte(caller))
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(nil)
}

func (t *MedicalRecordChaincode) updatePatient(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := PatientInfo{}
	_, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	_, err = t.getPatient(stub, params.PatientID)
	if err != nil {
		return shim.Error(err.Error())
	}
	t.setPatient(stub, params)
	return shim.Success(nil)
}

func (t *MedicalRecordChaincode) getPatientById(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := PatientIDParams{}
	caller, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	ac := AccessControl{t}
	access, _ := ac.checkAccess(stub, params.PatientID, params.Doctor, []byte(caller))
	if access == 0 {
		return shim.Error("Caller does not have access to patient info.")
	}
	patient, err := t.getPatient(stub, params.PatientID)
	if err != nil {
		return shim.Error(err.Error())
	}
	result, _ := json.Marshal(patient)
	return shim.Success(result)
}

func (t *MedicalRecordChaincode) patientVisit(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := VisitInfoParams{}
	_, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	countkey, _ := stub.CreateCompositeKey(VisitInfoCountKey, []string{fmt.Sprint(params.PatientID)})
	count, _ := t.getValue(stub, countkey)
	newVisit := MedicalVisit{
		VisitID:   count + 1,
		PatientID: params.PatientID,
		Doctor:    params.Doctor,
		Complaint: params.Complaint,
	}
	t.setVisit(stub, newVisit)
	t.setValue(stub, countkey, count+1)
	result, _ := json.Marshal(newVisit)
	return shim.Success(result)
}

func (t *MedicalRecordChaincode) setDiagnosis(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := DiagnosisParams{}
	caller, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	visit, _ := t.getVisit(stub, params.PatientID, params.VisitID)
	ac := AccessControl{t}
	access, _ := ac.checkAccess(stub, params.PatientID, visit.Doctor, []byte(caller))
	if access != 2 {
		return shim.Error("Caller does not have access to patient data.")
	}
	visit.Diagnosis = params.Diagnosis
	t.setVisit(stub, visit)
	result, _ := json.Marshal(visit)
	return shim.Success(result)
}

func (t *MedicalRecordChaincode) setPerscription(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := PerscriptionParams{}
	caller, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	visit, _ := t.getVisit(stub, params.PatientID, params.VisitID)
	ac := AccessControl{t}
	access, _ := ac.checkAccess(stub, params.PatientID, visit.Doctor, []byte(caller))
	if access != 2 {
		return shim.Error("Caller does not have access to patient data.")
	}
	visit.Perscription = params.Perscription
	t.setVisit(stub, visit)
	result, _ := json.Marshal(visit)
	return shim.Success(result)
}

func (t *MedicalRecordChaincode) getMedicalRecords(stub shim.ChaincodeStubInterface, args []string) peer.Response {
	params := PatientIDParams{}
	caller, _, err := getCallParams(stub, args, &params)
	if err != nil {
		return shim.Error(err.Error())
	}
	ac := AccessControl{t}
	access, _ := ac.checkAccess(stub, params.PatientID, params.Doctor, []byte(caller))
	if access != 2 {
		return shim.Error("Caller does not have access to patient info.")
	}
	countkey, _ := stub.CreateCompositeKey(VisitInfoCountKey, []string{fmt.Sprint(params.PatientID)})
	count, _ := t.getValue(stub, countkey)
	patient, err := t.getPatient(stub, params.PatientID)
	if err != nil {
		return shim.Error(err.Error())
	}
	history := []MedicalVisit{}
	for visitID := uint64(1); visitID <= count; visitID++ {
		visit, _ := t.getVisit(stub, params.PatientID, visitID)
		history = append(history, visit)
	}
	records := MedicalRecords{Patient: patient, History: history}
	result, _ := json.Marshal(records)
	return shim.Success(result)
}

// **************************************************************
// smart contract private functions
// **************************************************************

func (t *MedicalRecordChaincode) setPatient(stub shim.ChaincodeStubInterface, patient PatientInfo) error {
	key, _ := stub.CreateCompositeKey(PatientInfoKey, []string{fmt.Sprint(patient.PatientID)})
	result, _ := json.Marshal(patient)
	return stub.PutState(key, result)
}

func (t *MedicalRecordChaincode) getPatient(stub shim.ChaincodeStubInterface, id uint64) (PatientInfo, error) {
	var result PatientInfo
	key, _ := stub.CreateCompositeKey(PatientInfoKey, []string{fmt.Sprint(id)})
	data, err := stub.GetState(key)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (t *MedicalRecordChaincode) setVisit(stub shim.ChaincodeStubInterface, visit MedicalVisit) error {
	key, _ := stub.CreateCompositeKey(MedVisitKey, []string{fmt.Sprint(visit.PatientID), fmt.Sprint(visit.VisitID)})
	result, _ := json.Marshal(visit)
	return stub.PutState(key, result)
}

func (t *MedicalRecordChaincode) getVisit(stub shim.ChaincodeStubInterface, patientId uint64, id uint64) (MedicalVisit, error) {
	var result MedicalVisit
	key, _ := stub.CreateCompositeKey(MedVisitKey, []string{fmt.Sprint(patientId), fmt.Sprint(id)})
	data, err := stub.GetState(key)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

func (t *MedicalRecordChaincode) setValue(stub shim.ChaincodeStubInterface, key string, value uint64) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, value)
	return stub.PutState(key, data)
}

func (t *MedicalRecordChaincode) getValue(stub shim.ChaincodeStubInterface, key string) (uint64, error) {
	data, err := stub.GetState(key)
	if err != nil {
		return 0, err
	}

	// if the user cn is not in the state, then the balance is 0
	if data == nil {
		return 0, nil
	}
	return binary.LittleEndian.Uint64(data), nil
}

// **************************************************************
// Chaincode utils
// **************************************************************
func getCallParams(stub shim.ChaincodeStubInterface, args []string, params interface{}) (string, string, error) {
	if len(args) != 1 {
		return "", "", errors.New("Expected 1 argument")
	}
	err := json.Unmarshal([]byte(args[0]), params)
	if err != nil {
		return "", "", errors.New("Error parsing json")
	}
	caller, err := callerCN(stub)
	if err != nil {
		return "", "", errors.New("Error getting caller data")
	}
	owner, err := getOwnerCN(stub)
	if err != nil {
		return "", "", errors.New("Error getting owner data")
	}
	return caller, owner, nil
}

func getOwnerKey(stub shim.ChaincodeStubInterface) (string, error) {
	return stub.CreateCompositeKey(MetadataKey, []string{"Owner"})
}

func getOwnerCN(stub shim.ChaincodeStubInterface) (string, error) {
	key, _ := getOwnerKey(stub)
	data, err := stub.GetState(key)
	return string(data), err
}

//**************************************************************
// Utils
//**************************************************************

// CallerCN extracts caller certificate from calldata
func callerCN(stub shim.ChaincodeStubInterface) (string, error) {
	data, _ := stub.GetCreator()
	serializedID := msp.SerializedIdentity{}
	err := proto.Unmarshal(data, &serializedID)
	if err != nil {
		return "", errors.New("Could not unmarshal Creator")
	}
	cn, err := cnFromX509(string(serializedID.IdBytes))
	if err != nil {
		return "", err
	}
	return cn, nil
}

// extracts CN from an x509 certificate
func cnFromX509(certPEM string) (string, error) {
	cert, err := parsePEM(certPEM)
	if err != nil {
		return "", errors.New("Failed to parse certificate: " + err.Error())
	}
	return cert.Subject.CommonName, nil
}

func parsePEM(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("Failed to parse PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}
