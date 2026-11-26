package world

import "math"

// ---------- danger levels ----------
//
// Every spot of the world has a danger level: 0 inside a town's limits
// (safe), otherwise 1 near the spawn town rising to 10 × the world level on
// the far side of the globe. Creatures, ores, woods and fish get better (and
// worse) with it. It's a pure function of position, so the server, the
// in-browser engine and the maps all agree. A danger zone can set its own
// level (the Highland Moors are 3, right next to Manchester).

// MaxDanger is the highest danger level on this level's world.
func (w *World) MaxDanger() int { return 10 * max(1, w.Def.Level) }

// Danger is the danger level at block position (x,z).
func (w *World) Danger(x, z float64) int {
	for _, p := range w.Def.Places {
		if p.Zone == ZoneTown && w.Dist2(x, z, p.X, p.Z) <= p.Radius*p.Radius {
			return 0
		}
	}
	for _, zd := range w.Def.Zones {
		if zd.Danger > 0 && w.Dist2(x, z, zd.X, zd.Z) <= zd.Radius*zd.Radius {
			return min(zd.Danger, w.MaxDanger())
		}
	}
	return w.distanceDanger(x, z)
}

// distanceDanger is the danger from distance alone: the great-circle angle
// from the spawn town, 0..180 degrees, spread over 1..MaxDanger.
func (w *World) distanceDanger(x, z float64) int {
	sp := w.Def.Place(w.Def.Spawn)
	if sp == nil {
		return 1
	}
	lat, lon := w.Def.ToLatLon(x, z)
	a := greatCircle(sp.Lat, sp.Lon, lat, lon)
	return 1 + int(math.Round(float64(w.MaxDanger()-1)*a/180))
}

// ToLatLon is the inverse of LatLon: block position to latitude/longitude.
func (d *Def) ToLatLon(x, z float64) (lat, lon float64) {
	return 90 - z/float64(d.H)*180, x/float64(d.W)*360 - 180
}

// greatCircle is the angle in degrees between two points on the globe.
func greatCircle(lat1, lon1, lat2, lon2 float64) float64 {
	r := math.Pi / 180
	p1, p2 := lat1*r, lat2*r
	dp, dl := (lat2-lat1)*r, (lon2-lon1)*r
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * math.Asin(math.Min(1, math.Sqrt(h))) / r
}

// TownAt is the id of the town whose limits contain (x,z), or "".
func (w *World) TownAt(x, z float64) string {
	for _, p := range w.Def.Places {
		if p.Zone == ZoneTown && w.Dist2(x, z, p.X, p.Z) <= p.Radius*p.Radius {
			return p.ID
		}
	}
	return ""
}
