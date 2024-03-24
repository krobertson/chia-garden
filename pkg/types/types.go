package types

type PlotRequest struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type PlotResponse struct {
	Url string `json:"url"`
}

type PlotLocateRequest struct {
	Name string `json:"name"`
}

type PlotLocateResponse struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
	Sha1 string `json:"sha1sum"`
}
