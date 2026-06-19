package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/service/flight-search/internal/aggregator"
	"github.com/service/flight-search/internal/filter"
	"github.com/service/flight-search/internal/model"
)

type SearchHandler struct {
	aggregator *aggregator.Aggregator
}

func NewSearchHandler(agg *aggregator.Aggregator) *SearchHandler {
	return &SearchHandler{aggregator: agg}
}

func (h *SearchHandler) HandleSearch(c *gin.Context) {
	var req model.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	if err := validateRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	switch req.EffectiveTripType() {
	case "one_way":
		h.handleOneWay(c, req)
	case "round_trip":
		h.handleRoundTrip(c, req)
	case "multi_city":
		h.handleMultiCity(c, req)
	}
}

func (h *SearchHandler) handleOneWay(c *gin.Context, req model.SearchRequest) {
	flights, meta, cacheHit := h.aggregator.Search(c.Request.Context(), req)
	meta.CacheHit = cacheHit

	flights = filter.Apply(flights, req)
	meta.TotalResults = len(flights)

	resp := model.SearchResponse{
		SearchCriteria: model.SearchCriteria{
			Origin:        req.Origin,
			Destination:   req.Destination,
			DepartureDate: req.DepartureDate,
			Passengers:    req.Passengers,
			CabinClass:    req.CabinClass,
		},
		Metadata: meta,
		Flights:  flights,
	}

	if resp.Flights == nil {
		resp.Flights = []model.Flight{}
	}

	c.JSON(http.StatusOK, resp)
}

func (h *SearchHandler) handleRoundTrip(c *gin.Context, req model.SearchRequest) {
	start := time.Now()

	outboundLeg := aggregator.BuildLegRequest(req.Origin, req.Destination, req.DepartureDate, req)
	returnLeg := aggregator.BuildLegRequest(req.Destination, req.Origin, *req.ReturnDate, req)

	results := h.aggregator.SearchLegs(c.Request.Context(), []model.SearchRequest{outboundLeg, returnLeg})

	outbound := filter.Apply(results[0].Flights, req)
	returnFlights := filter.Apply(results[1].Flights, req)

	if outbound == nil {
		outbound = []model.Flight{}
	}
	if returnFlights == nil {
		returnFlights = []model.Flight{}
	}

	elapsed := time.Since(start)

	segMeta := []model.SegmentMetadata{
		aggregator.BuildSegmentMetadata(results[0].Metadata, req.Origin, req.Destination, len(outbound), results[0].CacheHit),
		aggregator.BuildSegmentMetadata(results[1].Metadata, req.Destination, req.Origin, len(returnFlights), results[1].CacheHit),
	}

	resp := model.RoundTripResponse{
		SearchCriteria: model.SearchCriteria{
			Origin:        req.Origin,
			Destination:   req.Destination,
			DepartureDate: req.DepartureDate,
			Passengers:    req.Passengers,
			CabinClass:    req.CabinClass,
		},
		Metadata: model.MultiMetadata{
			TotalResults:     len(outbound) + len(returnFlights),
			ProvidersQueried: results[0].Metadata.ProvidersQueried,
			SearchTimeMs:     elapsed.Milliseconds(),
			Segments:         segMeta,
		},
		OutboundFlights: outbound,
		ReturnFlights:   returnFlights,
	}

	c.JSON(http.StatusOK, resp)
}

func (h *SearchHandler) handleMultiCity(c *gin.Context, req model.SearchRequest) {
	start := time.Now()

	legs := make([]model.SearchRequest, len(req.Segments))
	for i, seg := range req.Segments {
		legs[i] = aggregator.BuildLegRequest(seg.Origin, seg.Destination, seg.DepartureDate, req)
	}

	results := h.aggregator.SearchLegs(c.Request.Context(), legs)

	segments := make(map[string]model.SegmentResult, len(req.Segments))
	segMetas := make([]model.SegmentMetadata, len(req.Segments))
	totalResults := 0

	for i, seg := range req.Segments {
		flights := filter.Apply(results[i].Flights, req)
		if flights == nil {
			flights = []model.Flight{}
		}
		route := fmt.Sprintf("%s→%s", seg.Origin, seg.Destination)

		segments[strconv.Itoa(i)] = model.SegmentResult{
			Route:   route,
			Flights: flights,
		}
		segMetas[i] = aggregator.BuildSegmentMetadata(results[i].Metadata, seg.Origin, seg.Destination, len(flights), results[i].CacheHit)
		totalResults += len(flights)
	}

	elapsed := time.Since(start)

	providersQueried := 0
	if len(results) > 0 {
		providersQueried = results[0].Metadata.ProvidersQueried
	}

	resp := model.MultiCityResponse{
		SearchCriteria: model.MultiCitySearchCriteria{
			TripType:   "multi_city",
			Passengers: req.Passengers,
			CabinClass: req.CabinClass,
			Segments:   req.Segments,
		},
		Metadata: model.MultiMetadata{
			TotalResults:     totalResults,
			ProvidersQueried: providersQueried,
			SearchTimeMs:     elapsed.Milliseconds(),
			Segments:         segMetas,
		},
		Segments: segments,
	}

	c.JSON(http.StatusOK, resp)
}
