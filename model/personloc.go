package model

type PersonLoc struct {
	Cardnum     string `json:"cardnum"`
	DevNum      string `json:"devNum"`
	Isinwell    string `json:"isinwell"`
	Intime      string `json:"intime"`
	Outtime     string `json:"outtime"`
	CardType    string `json:"cardType"`
	X           string `json:"x"`
	Y           string `json:"y"`
	DevTime     string `json:"devTime"`
	Distance    string `json:"distance"`
	Description string `json:"description"`
	Direction   string `json:"direction"`
}

type WorkSite struct {
	Number       string `json:"number"`
	Name         string `json:"name"`
	WorkSiteType string `json:"workSiteType"`
	Address      string `json:"address"`
	X            string `json:"x"`
	Y            string `json:"y"`
	Direction    string `json:"direction"`
	WorkAreaName string `json:"workAreaName"`
	DevType      string `json:"DevType"`
}

type WorkArea struct {
	Id               string `json:"id"`
	Name             string `json:"name"`
	ParentId         string `json:"parentId"`
	TypeId           string `json:"typeId"`
	WorkAreaTypeName string `json:"workAreaTypeName"`
	PersonSize       string `json:"personSize"`
	VehicleSize      string `json:"vehicleSize"`
}

type Person struct {
	Name           string `json:"name"`
	CardNumber     string `json:"cardNumber"`
	EmpNo          string `json:"empNo"`
	Sex            string `json:"sex"`
	Department     string `json:"department"`
	TypeOfWork     string `json:"typeOfWork"`
	OfficePosition string `json:"officePosition"`
	Id             string `json:"Id"`
	IsCadres       string `json:"isCadres"`
}
