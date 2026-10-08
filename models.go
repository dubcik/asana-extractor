package main

type User struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type,omitempty"`
}

type Project struct {
	GID             string `json:"gid"`
	Name            string `json:"name"`
	ResourceSubtype string `json:"resource_subtype,omitempty"`
	ResourceType    string `json:"resource_type,omitempty"`
}
