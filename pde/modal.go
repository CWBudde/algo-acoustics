package pde

import (
	"math"
	sortpkg "sort"

	"github.com/cwbudde/algo-acoustics/acoustics"
	"github.com/cwbudde/algo-acoustics/scene"
)

// ModalFrequency identifies a theoretical shoebox mode.
type ModalFrequency struct {
	Freq       float64
	Nx, Ny, Nz int
}

// ShoeboxModes returns analytical shoebox modal frequencies up to maxOrder.
func ShoeboxModes(room *scene.Shoebox, maxOrder int) []ModalFrequency {
	if room == nil || maxOrder < 0 {
		return nil
	}

	modes := make([]ModalFrequency, 0)

	for nx := 0; nx <= maxOrder; nx++ {
		for ny := 0; ny <= maxOrder; ny++ {
			for nz := 0; nz <= maxOrder; nz++ {
				if nx == 0 && ny == 0 && nz == 0 {
					continue
				}

				freq := acoustics.SpeedOfSound / 2 * math.Sqrt(
					math.Pow(float64(nx)/room.Width, 2)+
						math.Pow(float64(ny)/room.Depth, 2)+
						math.Pow(float64(nz)/room.Height, 2),
				)
				modes = append(modes, ModalFrequency{Freq: freq, Nx: nx, Ny: ny, Nz: nz})
			}
		}
	}

	sortpkg.Slice(modes, func(i, j int) bool { return modes[i].Freq < modes[j].Freq })

	return modes
}
