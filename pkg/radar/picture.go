package radar

import (
	"math"
	"slices"

	"github.com/dharmab/skyeye/pkg/brevity"
	"github.com/dharmab/skyeye/pkg/coalitions"
	"github.com/dharmab/skyeye/pkg/spatial"
	"github.com/martinlindhe/unit"
	"github.com/rs/zerolog/log"
)

// Picture returns a picture of the radar scope anchored at the center point, within the given radius,
// filtered by the given coalition and contact category. The first return value is the total number of groups
// and the second is a slice of up to 3 high priority groups. Each group has Bullseye set relative to the
// point provided in SetBullseye.
func (r *Radar) Picture(radius unit.Length, coalition coalitions.Coalition, filter brevity.ContactCategory) (int, []brevity.Group) {
	// Find groups near the center point
	r.centerLock.RLock()
	defer r.centerLock.RUnlock()
	origin := r.center
	if spatial.IsZero(origin) {
		log.Warn().Msg("center point is not set yet, using bullseye")
		origin = r.Bullseye(coalition)
		if spatial.IsZero(origin) {
			log.Warn().Msg("bullseye point is not yet set, picture will be incoherent")
		}
	}

	groups := r.findNearbyGroups(
		origin,
		0,
		math.MaxFloat64,
		radius,
		coalition,
		filter,
		[]uint64{},
	)

	// Sort groups from highest to lowest threat
	slices.SortFunc(groups, r.compareThreat)

	// Return the top 3 groups
	capacity := 3
	if len(groups) < capacity {
		capacity = len(groups)
	}
	result := make([]brevity.Group, capacity)
	for i := range capacity {
		result[i] = groups[i]
	}
	log.Info().Float64("centerLat", origin.Lat()).Float64("centerLon", origin.Lon()).Int("groups", len(groups)).Msg("generating PICTURE")
	return len(groups), result
}

func (r *Radar) compareThreat(a, b *group) int {
	aIsHigherThreat := -1
	bIsHigherThreat := 1

	// Prioritize fixed-wing aircraft over rotary-wing aircraft
	aIsHelo := a.category() == brevity.RotaryWing
	bIsHelo := b.category() == brevity.RotaryWing
	if !aIsHelo && bIsHelo {
		return aIsHigherThreat
	} else if aIsHelo && !bIsHelo {
		return bIsHigherThreat
	}

	// Prioritize aircraft within threat radius over aircraft outside threat radius
	distanceA := spatial.Distance(r.center, a.point())
	distanceB := spatial.Distance(r.center, b.point())
	aIsThreat := distanceA < a.threatRadius()
	bIsThreat := distanceB < b.threatRadius()
	if aIsThreat && !bIsThreat {
		return aIsHigherThreat
	} else if !aIsThreat && bIsThreat {
		return bIsHigherThreat
	}

	// Prioritize fighters within threat radius
	if aIsThreat && bIsThreat {
		aIsFighter := a.isFighter()
		bIsFighter := b.isFighter()
		if aIsFighter && !bIsFighter {
			return aIsHigherThreat
		} else if !aIsFighter && bIsFighter {
			return bIsHigherThreat
		}
	}

	// Compare distance relative to threat radius
	weightedDistanceA := weightedDistance(distanceA, a.threatRadius())
	weightedDistanceB := weightedDistance(distanceB, b.threatRadius())
	if math.Abs(weightedDistanceA.NauticalMiles()-weightedDistanceB.NauticalMiles()) > 3 {
		if weightedDistanceA < weightedDistanceB {
			return aIsHigherThreat
		}
		return bIsHigherThreat
	}

	// Compare absolute distance
	if math.Abs(distanceA.NauticalMiles()-distanceB.NauticalMiles()) > 3 {
		if distanceA < distanceB {
			return aIsHigherThreat
		} else if distanceA > distanceB {
			return bIsHigherThreat
		}
	}

	// Compare altitude
	if a.Altitude() > b.Altitude() {
		return aIsHigherThreat
	} else if a.Altitude() < b.Altitude() {
		return bIsHigherThreat
	}

	return 0
}

func weightedDistance(distance unit.Length, threatRadius unit.Length) unit.Length {
	if distance > threatRadius {
		distance = threatRadius
	}
	return (distance / threatRadius) * distance
}
