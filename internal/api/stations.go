package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"heatpulse/internal/telemetry"
)

func (a *App) listStations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.Stations.List())
}

func (a *App) beginExchangerSwitch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	operation, err := a.ExchangerSwitch.Begin(chi.URLParam(r, "stationID"), input.Source, input.Target)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (a *App) recordFlowReceipt(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ExchangerID  string  `json:"exchanger_id"`
		OperationID  string  `json:"operation_id"`
		LitersSecond float64 `json:"liters_second"`
		Stable       bool    `json:"stable"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	receipt := telemetry.FlowReceipt{
		StationID: chi.URLParam(r, "stationID"), ExchangerID: input.ExchangerID,
		OperationID: input.OperationID, LitersSecond: input.LitersSecond, Stable: input.Stable,
	}
	if err := a.FlowReceipts.Record(receipt); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (a *App) advanceExchangerSwitch(w http.ResponseWriter, r *http.Request) {
	operation, err := a.ExchangerSwitch.Advance(chi.URLParam(r, "operationID"))
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, operation)
}

func (a *App) startManualValve(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ValveID string  `json:"valve_id"`
		Target  float64 `json:"target"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	operation, err := a.Manual.MoveValve(chi.URLParam(r, "stationID"), input.ValveID, input.Target)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operation)
}

func (a *App) cancelManualValve(w http.ResponseWriter, r *http.Request) {
	operation, err := a.Manual.Cancel(chi.URLParam(r, "operationID"))
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, operation)
}

func (a *App) failManualValve(w http.ResponseWriter, r *http.Request) {
	operation, err := a.ValveOperations.Fail(chi.URLParam(r, "operationID"))
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, operation)
}

func (a *App) openFrostValve(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ValveID string `json:"valve_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	operation, err := a.Frost.OpenValve(chi.URLParam(r, "stationID"), input.ValveID)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, operation)
}
