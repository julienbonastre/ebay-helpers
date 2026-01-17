package calculator

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ShippingResult holds the complete calculation breakdown
type ShippingResult struct {
	Inputs    ShippingInputs    `json:"inputs"`
	Breakdown ShippingBreakdown `json:"breakdown"`
	Total     float64           `json:"totalShipping"`
	Warnings  ShippingWarnings  `json:"warnings"`
}

// ShippingInputs captures the input parameters
type ShippingInputs struct {
	ItemValueAUD      float64 `json:"itemValueAUD"`
	WeightBand        string  `json:"weightBand"`
	BrandName         string  `json:"brandName"`
	CountryOfOrigin   string  `json:"countryOfOrigin"`
	TariffRate        float64 `json:"tariffRate"`
	IncludeExtraCover bool    `json:"includeExtraCover"`
	DiscountBand      int     `json:"discountBand"`
}

// ShippingBreakdown shows individual cost components
type ShippingBreakdown struct {
	AusPostShipping  float64 `json:"ausPostShipping"`
	ExtraCover       float64 `json:"extraCover"`
	ShippingSubtotal float64 `json:"shippingSubtotal"`
	TariffDuties     float64 `json:"tariffDuties"`
	ZonosFees        float64 `json:"zonosFees"`
	DutiesSubtotal   float64 `json:"dutiesSubtotal"`
}

// ShippingWarnings holds any warnings for the user
type ShippingWarnings struct {
	ExtraCoverRecommended bool `json:"extraCoverRecommended"`
}

// GetCountryOfOrigin returns the COO for a brand, or default
func GetCountryOfOrigin(brandName string) string {
	if brand, ok := Brands[brandName]; ok {
		return brand.PrimaryCOO
	}
	return DefaultCOO
}

// GetTariffRate returns the US tariff rate for a country
func GetTariffRate(country string) float64 {
	if rate, ok := USATariffs.Rates[country]; ok {
		return rate
	}
	return USATariffs.Rates[DefaultCOO]
}

// CalculateAusPostShipping calculates the AusPost shipping cost
func CalculateAusPostShipping(zone, weightBand string, discountBand int) (float64, error) {
	zoneData, ok := PostalZones[zone]
	if !ok {
		return 0, fmt.Errorf("unknown zone: %s", zone)
	}

	weightData, ok := zoneData.WeightBands[weightBand]
	if !ok {
		return 0, fmt.Errorf("unknown weight band: %s", weightBand)
	}

	discount, ok := zoneData.DiscountBands[discountBand]
	if !ok {
		discount = 0
	}

	// Formula: Base × (1 + handling) × (1 - discount)
	withHandling := weightData.BasePrice * (1 + zoneData.HandlingFee)
	finalPrice := withHandling * (1 - discount)

	return round2(finalPrice), nil
}

// CalculateExtraCover calculates insurance cost
func CalculateExtraCover(itemValueAUD float64, discountBand int) float64 {
	if itemValueAUD <= ExtraCover.ThresholdAUD {
		return 0
	}

	discount, ok := ExtraCover.DiscountBands[discountBand]
	if !ok {
		discount = 0
	}

	// Formula: (ItemValue - 100) / 100 × $4 × (1 - discount)
	coverUnits := (itemValueAUD - ExtraCover.ThresholdAUD) / 100
	cost := coverUnits * ExtraCover.BasePricePer100 * (1 - discount)

	return round2(cost)
}

// CalculateTariffDuties calculates US import duties
func CalculateTariffDuties(itemValueAUD float64, countryOfOrigin string) float64 {
	rate := GetTariffRate(countryOfOrigin)
	return round2(itemValueAUD * rate)
}

// CalculateZonosFees calculates Zonos processing fees
func CalculateZonosFees(tariffAmount float64) float64 {
	percentageFee := tariffAmount * Zonos.ProcessingChargePercent
	total := percentageFee + Zonos.FlatFeeAUD
	return round2(total)
}

// ShouldWarnExtraCover returns true if extra cover warning should show
func ShouldWarnExtraCover(itemValueAUD float64, hasExtraCover bool) bool {
	return itemValueAUD >= ExtraCover.WarningThresholdAUD && !hasExtraCover
}

// CalculateUSAShippingParams holds parameters for the main calculation
type CalculateUSAShippingParams struct {
	ItemValueAUD      float64
	WeightBand        string
	BrandName         string
	CountryOfOrigin   string // optional override
	IncludeExtraCover bool
	DiscountBand      int
}

// CalculateUSAShipping performs the complete shipping calculation
func CalculateUSAShipping(params CalculateUSAShippingParams) (*ShippingResult, error) {
	zone := "3-USA & Canada"

	// Determine country of origin
	coo := params.CountryOfOrigin
	if coo == "" {
		coo = GetCountryOfOrigin(params.BrandName)
	}
	tariffRate := GetTariffRate(coo)

	// Calculate components
	ausPostShipping, err := CalculateAusPostShipping(zone, params.WeightBand, params.DiscountBand)
	if err != nil {
		return nil, err
	}

	var extraCover float64
	if params.IncludeExtraCover {
		extraCover = CalculateExtraCover(params.ItemValueAUD, params.DiscountBand)
	}

	tariffDuties := CalculateTariffDuties(params.ItemValueAUD, coo)
	zonosFees := CalculateZonosFees(tariffDuties)

	shippingSubtotal := ausPostShipping + extraCover
	dutiesSubtotal := tariffDuties + zonosFees
	total := shippingSubtotal + dutiesSubtotal

	return &ShippingResult{
		Inputs: ShippingInputs{
			ItemValueAUD:      params.ItemValueAUD,
			WeightBand:        params.WeightBand,
			BrandName:         params.BrandName,
			CountryOfOrigin:   coo,
			TariffRate:        tariffRate,
			IncludeExtraCover: params.IncludeExtraCover,
			DiscountBand:      params.DiscountBand,
		},
		Breakdown: ShippingBreakdown{
			AusPostShipping:  ausPostShipping,
			ExtraCover:       extraCover,
			ShippingSubtotal: shippingSubtotal,
			TariffDuties:     tariffDuties,
			ZonosFees:        zonosFees,
			DutiesSubtotal:   dutiesSubtotal,
		},
		Total: round2(total),
		Warnings: ShippingWarnings{
			ExtraCoverRecommended: ShouldWarnExtraCover(params.ItemValueAUD, params.IncludeExtraCover),
		},
	}, nil
}

// GetWeightBandFromGrams returns the weight band for a given weight
func GetWeightBandFromGrams(weightGrams int) string {
	switch {
	case weightGrams < 250:
		return "XSmall"
	case weightGrams < 500:
		return "Small"
	case weightGrams < 1000:
		return "Medium"
	case weightGrams < 1500:
		return "Large"
	default:
		return "XLarge"
	}
}

// GetAvailableBrands returns all brand names sorted
func GetAvailableBrands() []string {
	brands := make([]string, 0, len(Brands))
	for name := range Brands {
		brands = append(brands, name)
	}
	sort.Strings(brands)
	return brands
}

// WeightBandInfo holds weight band details for API responses
type WeightBandInfo struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	MaxWeight int     `json:"maxWeight"`
	BasePrice float64 `json:"basePrice"`
}

// GetWeightBands returns all weight bands for USA zone
func GetWeightBands() []WeightBandInfo {
	zone := PostalZones["3-USA & Canada"]
	bands := make([]WeightBandInfo, 0, len(zone.WeightBands))

	// Order matters for display
	order := []string{"XSmall", "Small", "Medium", "Large", "XLarge"}
	for _, key := range order {
		if wb, ok := zone.WeightBands[key]; ok {
			bands = append(bands, WeightBandInfo{
				Key:       key,
				Label:     wb.Label,
				MaxWeight: wb.MaxWeight,
				BasePrice: wb.BasePrice,
			})
		}
	}
	return bands
}

// TariffCountryInfo holds tariff info for API responses
type TariffCountryInfo struct {
	Country     string  `json:"country"`
	Rate        float64 `json:"rate"`
	RatePercent int     `json:"ratePercent"`
}

// GetTariffCountries returns all countries with tariff rates
func GetTariffCountries() []TariffCountryInfo {
	countries := make([]TariffCountryInfo, 0, len(USATariffs.Rates))
	for country, rate := range USATariffs.Rates {
		countries = append(countries, TariffCountryInfo{
			Country:     country,
			Rate:        rate,
			RatePercent: int(math.Round(rate * 100)),
		})
	}
	sort.Slice(countries, func(i, j int) bool {
		return countries[i].Country < countries[j].Country
	})
	return countries
}

// round2 rounds to 2 decimal places
func round2(val float64) float64 {
	return math.Round(val*100) / 100
}

// ZoneShippingResult holds calculation results for a single zone
type ZoneShippingResult struct {
	ZoneID      string            `json:"zoneId"`      // e.g., "1-New Zealand"
	ZoneName    string            `json:"zoneName"`    // e.g., "New Zealand"
	Inputs      ShippingInputs    `json:"inputs"`
	Breakdown   ShippingBreakdown `json:"breakdown"`
	Total       float64           `json:"totalShipping"`
	Warnings    ShippingWarnings  `json:"warnings"`
	HasTariffs  bool              `json:"hasTariffs"`  // Whether this zone applies tariffs
}

// MultiZoneResult holds calculation results for all zones
type MultiZoneResult struct {
	Zones []ZoneShippingResult `json:"zones"`
}

// CalculateAllZonesParams holds parameters for multi-zone calculation
type CalculateAllZonesParams struct {
	ItemValueAUD      float64
	WeightBand        string
	BrandName         string
	CountryOfOrigin   string // optional override
	IncludeExtraCover bool
	DiscountBand      int
}

// CalculateAllZones performs shipping calculation for all zones
func CalculateAllZones(params CalculateAllZonesParams) (*MultiZoneResult, error) {
	// Determine country of origin
	coo := params.CountryOfOrigin
	if coo == "" {
		coo = GetCountryOfOrigin(params.BrandName)
	}

	// Get all zones in a consistent order
	zoneOrder := []string{"1-New Zealand", "3-USA & Canada", "4-UK & Ireland"}
	results := make([]ZoneShippingResult, 0, len(zoneOrder))

	for _, zoneID := range zoneOrder {
		_, ok := PostalZones[zoneID]
		if !ok {
			continue // Skip if zone not found
		}

		// Determine if this zone has tariffs (only USA)
		hasTariffs := zoneID == "3-USA & Canada"

		// Calculate components
		ausPostShipping, err := CalculateAusPostShipping(zoneID, params.WeightBand, params.DiscountBand)
		if err != nil {
			return nil, fmt.Errorf("zone %s: %w", zoneID, err)
		}

		var extraCover float64
		if params.IncludeExtraCover {
			extraCover = CalculateExtraCover(params.ItemValueAUD, params.DiscountBand)
		}

		shippingSubtotal := ausPostShipping + extraCover

		// Calculate tariffs and duties (only for USA)
		var tariffDuties, zonosFees, dutiesSubtotal float64
		var tariffRate float64
		if hasTariffs {
			tariffRate = GetTariffRate(coo)
			tariffDuties = CalculateTariffDuties(params.ItemValueAUD, coo)
			zonosFees = CalculateZonosFees(tariffDuties)
			dutiesSubtotal = tariffDuties + zonosFees
		}

		total := shippingSubtotal + dutiesSubtotal

		// Extract zone name from zone ID (e.g., "1-New Zealand" -> "New Zealand")
		zoneName := zoneID
		if idx := strings.Index(zoneID, "-"); idx >= 0 && idx < len(zoneID)-1 {
			zoneName = zoneID[idx+1:]
		}

		results = append(results, ZoneShippingResult{
			ZoneID:   zoneID,
			ZoneName: zoneName,
			Inputs: ShippingInputs{
				ItemValueAUD:      params.ItemValueAUD,
				WeightBand:        params.WeightBand,
				BrandName:         params.BrandName,
				CountryOfOrigin:   coo,
				TariffRate:        tariffRate,
				IncludeExtraCover: params.IncludeExtraCover,
				DiscountBand:      params.DiscountBand,
			},
			Breakdown: ShippingBreakdown{
				AusPostShipping:  ausPostShipping,
				ExtraCover:       extraCover,
				ShippingSubtotal: shippingSubtotal,
				TariffDuties:     tariffDuties,
				ZonosFees:        zonosFees,
				DutiesSubtotal:   dutiesSubtotal,
			},
			Total: round2(total),
			Warnings: ShippingWarnings{
				ExtraCoverRecommended: ShouldWarnExtraCover(params.ItemValueAUD, params.IncludeExtraCover),
			},
			HasTariffs: hasTariffs,
		})
	}

	return &MultiZoneResult{
		Zones: results,
	}, nil
}
