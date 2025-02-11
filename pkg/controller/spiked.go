package controller

import (
	"context"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/martinlindhe/unit"
	"github.com/rs/zerolog/log"
)

// HandleSpiked handles a SPIKED request by reporting any enemy groups in the direction of the radar spike.
func (c *Controller) HandleSpiked(ctx context.Context, request *brevity.SpikedRequest) {
	logger := log.With().Str("callsign", request.Callsign).Type("type", request).Float64("bearing", request.Bearing.Degrees()).Logger()
	logger.Debug().Msg("handling request")

	if !request.Bearing.IsMagnetic() {
		logger.Error().Stringer("bearing", request.Bearing).Msg("bearing provided to HandleSpiked should be magnetic")
	}

	foundCallsign, trackfile, ok := c.findCallsign(request.Callsign)
	if !ok {
		c.calls <- NewCall(ctx, brevity.NegativeRadarContactResponse{Callsign: request.Callsign})
		return
	}

	origin := trackfile.LastKnown().Point
	arc := unit.Angle(30) * unit.Degree
	distance := unit.Length(120) * unit.NauticalMile
	nearestGroup := c.scope.FindNearestGroupInSector(
		origin,
		lowestAltitude,
		highestAltitude,
		distance,
		request.Bearing,
		arc,
		c.coalition.Opposite(),
		brevity.FixedWing,
	)

	if nearestGroup == nil {
		logger.Info().Msg("no hostile groups found within spike cone")
		c.calls <- NewCall(ctx, brevity.SpikedResponseV2{
			Callsign: foundCallsign,
			Status:   false,
			Bearing:  request.Bearing,
		})
		return
	}
	nearestGroup.SetDeclaration(brevity.Hostile)

	logger = logger.With().Stringer("group", nearestGroup).Logger()
	logger.Debug().Msg("hostile group found within spike cone")
	c.calls <- NewCall(ctx, brevity.SpikedResponseV2{
		Callsign: foundCallsign,
		Status:   true,
		Bearing:  request.Bearing,
		Group:    nearestGroup,
	})
}
