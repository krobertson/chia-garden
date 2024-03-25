// Copyright © 2024 Ken Robertson <ken@invalidlogic.com>

package types

type PlotRequest struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type PlotResponse struct {
	Hostname string `json:"hostname"`
	Store    string `json:"store"`
	Url      string `json:"url"`
}

type PlotLocateRequest struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type PlotLocateResponse struct {
	Hostname string `json:"hostname"`
}
