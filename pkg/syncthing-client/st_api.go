package syncthingclient

import (
	"errors"
)

type UseageReport int

const (
	AllowUsageReport UseageReport = 3
	DenyUsageReport  UseageReport = -1
)

// === Start Syncthing API response mappings ===
type StConfig struct {
	Gui     GuiElement
	Options OptionsElement
	Devices []DeviceElement
	Folders []FolderElement
}

type GuiElement struct {
	Enabled, UseTls, InsecureAdminAccess, Debugging, SkipHostCheck, InsecureAllowFrameLoading bool
	Address, AuthMode, User, Password                                                         string
}

type OptionsElement struct {
	UrAccepted               UseageReport
	MaxSendKbps, MaxRecvKbps int
}
type DeviceElement struct {
	DeviceId, Name, Compression, CertName, IntroducedBy                        string
	Addresses, IgnoredFolders, AllowedNetworks                                 []string
	Introducer, SkipIntroductionRemovals, Paused, AutoAcceptFolders, Untrusted bool
	MaxSendKbps, MaxRecvKbps                                                   int
}
type FolderElement struct {
	Id, Label, FilesystemType, Path, Type, Order, MarkerName string
	Devices                                                  []DeviceReference
	IgnorePerms, IgnoreDelete, Paused                        bool
	RescanIntervalS                                          int
}
type DeviceReference struct {
	DeviceId, IntroducedBy, EncryptionPassword string
}

// === End Syncthing API response mappings ===

func (folder *FolderElement) AddDeviceById(new_id string) bool {
	for _, device_ref := range folder.Devices {
		if device_ref.DeviceId == new_id {
			return false
		}
	}
	folder.Devices = append(folder.Devices, DeviceReference{DeviceId: new_id})
	return true
}

// func (folder FolderElement) ResolveDeviceNameToID(new_id string) bool {
// 	return true
// }

func (c StClient) GetDeviceIndexById(list []DeviceElement, id string) int {
	for i, device := range list {
		if device.DeviceId == id {
			return i
		}
	}
	return -1
}
func (c StClient) GetFolderIndexById(list []FolderElement, id string) int {
	for i, folder := range list {
		if folder.Id == id {
			return i
		}
	}
	return -1
}

// Connection Check
func (c StClient) Ping() (bool, string) {
	pingReq, _ := c.newRequestTemplate("GET", "/rest/system/ping", nil)
	_, err := c.do(pingReq, nil)

	if err != nil {
		return false, err.Error()
	}

	return true, "Success"

}

// Get a Config Dump
func (c StClient) GetConfig() (StConfig, error) {
	config := StConfig{}
	request, _ := c.newRequestTemplate("GET", "/rest/config", nil)
	_, err := c.do(request, &config)
	return config, err
}

// === Start config options ===
func (c StClient) SendUsageStatistics(sendIt UseageReport) error {
	value := struct {
		UrAccepted int
	}{
		UrAccepted: int(sendIt),
	}
	req, err := c.newRequestTemplate("PATCH", "/rest/config/options", value)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	} else {
		return errors.New("Syncthing returned: " + response.Status)
	}
}
func (c StClient) SetSpeed(send int64, receive int64) error {
	newSettings := struct {
		MaxSendKbps, MaxRecvKbps int
	}{
		MaxSendKbps: int(send),
		MaxRecvKbps: int(receive),
	}
	req, err := c.newRequestTemplate("PATCH", "/rest/config/options", newSettings)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	} else {
		return errors.New("Syncthing returned: " + response.Status)
	}
}

func (c StClient) SetAuth(enabled bool, username string, password string) error {
	newSettings := struct {
		InsecureAdminAccess      bool
		AuthMode, User, Password string
	}{
		InsecureAdminAccess: enabled,
		AuthMode:            "static",
		User:                username,
		Password:            password,
	}
	req, err := c.newRequestTemplate("PATCH", "/rest/config/gui", newSettings)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	}
	if response.StatusCode == 200 {
		return nil
	} else {
		return errors.New("Syncthing returned: " + response.Status)
	}
}

// === End config options ===

func (c StClient) ReplaceDevice(dev DeviceElement) error {
	req, err := c.newRequestTemplate("PUT", "/rest/config/devices/"+dev.DeviceId, dev)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("PUT Request returned unexpected status: " + response.Status)
	}

	return nil
}
func (c StClient) DeleteDevice(deviceId string) error {
	req, _ := c.newRequestTemplate("DELETE", "/rest/config/devices/"+deviceId, nil)
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("DELETE Request returned unexpected status: " + response.Status)
	}
	return nil
}

func (c StClient) ReplaceFolder(folder FolderElement) error {
	req, err := c.newRequestTemplate("PUT", "/rest/config/folders/"+folder.Id, folder)
	if err != nil {
		return err
	}
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("PUT Request returned unexpected status: " + response.Status)
	}

	return nil
}
func (c StClient) DeleteFolder(folderId string) error {
	req, _ := c.newRequestTemplate("DELETE", "/rest/config/folders/"+folderId, nil)
	response, err := c.do(req, nil)
	if err != nil {
		return err
	} else if response.StatusCode != 200 {
		return errors.New("DELETE Request returned unexpected status: " + response.Status)
	}
	return nil
}
