/**
 * Postage Calculator - Implements tariff and shipping cost calculations
 * Based on Australia Post rates, US IEEPA tariffs, and Zonos processing fees
 */

const postalRates = require('../data/postalRates.json');
const tariffs = require('../data/tariffs.json');
const brands = require('../data/brands.json');
const extraCoverData = require('../data/extraCover.json');

/**
 * Get the country of origin for a brand
 * @param {string} brandName - The brand name
 * @returns {string} - Country of origin
 */
function getCountryOfOrigin(brandName) {
  const brand = brands.brands[brandName];
  if (brand) {
    return brand.primaryCOO;
  }
  return brands.defaultCOO; // Default to China if unknown
}

/**
 * Get the tariff rate for a country
 * @param {string} country - Country of origin
 * @returns {number} - Tariff rate as decimal (e.g., 0.20 for 20%)
 */
function getTariffRate(country) {
  return tariffs.usaTariffs.rates[country] ?? tariffs.usaTariffs.rates['China'];
}

/**
 * Calculate AusPost shipping cost
 * @param {string} zone - Destination zone (e.g., "3-USA & Canada")
 * @param {string} weightBand - Weight category (XSmall, Small, Medium, Large, XLarge)
 * @param {number} discountBand - Discount band (0-5)
 * @returns {number} - Shipping cost in AUD
 */
function calculateAusPostShipping(zone, weightBand, discountBand = 0) {
  const zoneData = postalRates.zones[zone];
  if (!zoneData) {
    throw new Error(`Unknown zone: ${zone}`);
  }

  const weightData = zoneData.weightBands[weightBand];
  if (!weightData) {
    throw new Error(`Unknown weight band: ${weightBand}`);
  }

  const basePrice = weightData.basePrice;
  const handlingFee = zoneData.handlingFee;
  const discount = zoneData.discountBands[discountBand] || 0;

  // Formula: Base × (1 + handling) × (1 - discount)
  const withHandling = basePrice * (1 + handlingFee);
  const finalPrice = withHandling * (1 - discount);

  return Math.round(finalPrice * 100) / 100;
}

/**
 * Calculate extra cover (insurance) cost
 * @param {number} itemValueAUD - Item value in AUD
 * @param {number} discountBand - Discount band (0-5)
 * @returns {number} - Extra cover cost in AUD
 */
function calculateExtraCover(itemValueAUD, discountBand = 0) {
  const { basePricePer100, thresholdAUD, discountBands } = extraCoverData.extraCover;

  if (itemValueAUD <= thresholdAUD) {
    return 0;
  }

  const discount = discountBands[discountBand] || 0;
  // Formula: (ItemValue - 100) / 100 × $4 × (1 - discount)
  const coverUnits = (itemValueAUD - thresholdAUD) / 100;
  const cost = coverUnits * basePricePer100 * (1 - discount);

  return Math.round(cost * 100) / 100;
}

/**
 * Calculate US tariff duties
 * @param {number} itemValueAUD - Item value in AUD
 * @param {string} countryOfOrigin - Country of manufacture
 * @returns {number} - Tariff amount in AUD
 */
function calculateTariffDuties(itemValueAUD, countryOfOrigin) {
  const tariffRate = getTariffRate(countryOfOrigin);
  return Math.round(itemValueAUD * tariffRate * 100) / 100;
}

/**
 * Calculate Zonos processing fees
 * @param {number} tariffAmount - The tariff duties amount in AUD
 * @returns {number} - Zonos fees in AUD
 */
function calculateZonosFees(tariffAmount) {
  const { processingChargePercent, flatFeeAUD } = tariffs.zonos;
  const percentageFee = tariffAmount * processingChargePercent;
  const total = percentageFee + flatFeeAUD;
  return Math.round(total * 100) / 100;
}

/**
 * Check if extra cover warning should be shown
 * @param {number} itemValueAUD - Item value in AUD
 * @param {boolean} hasExtraCover - Whether extra cover is enabled
 * @returns {boolean} - True if warning should be shown
 */
function shouldWarnExtraCover(itemValueAUD, hasExtraCover) {
  return itemValueAUD >= extraCoverData.extraCover.warningThresholdAUD && !hasExtraCover;
}

/**
 * Calculate total shipping cost to USA including all fees
 * @param {Object} params - Calculation parameters
 * @param {number} params.itemValueAUD - Item value in AUD
 * @param {string} params.weightBand - Weight category
 * @param {string} params.brandName - Brand name (for COO lookup)
 * @param {string} [params.countryOfOrigin] - Override COO (optional)
 * @param {boolean} [params.includeExtraCover=false] - Include insurance
 * @param {number} [params.discountBand=0] - AusPost discount band
 * @returns {Object} - Detailed breakdown of costs
 */
function calculateUSAShipping(params) {
  const {
    itemValueAUD,
    weightBand,
    brandName,
    countryOfOrigin: overrideCOO,
    includeExtraCover = false,
    discountBand = 0
  } = params;

  const zone = '3-USA & Canada';
  const countryOfOrigin = overrideCOO || getCountryOfOrigin(brandName);
  const tariffRate = getTariffRate(countryOfOrigin);

  // Calculate components
  const ausPostShipping = calculateAusPostShipping(zone, weightBand, discountBand);
  const extraCover = includeExtraCover ? calculateExtraCover(itemValueAUD, discountBand) : 0;
  const tariffDuties = calculateTariffDuties(itemValueAUD, countryOfOrigin);
  const zonosFees = calculateZonosFees(tariffDuties);

  const shippingSubtotal = ausPostShipping + extraCover;
  const dutiesSubtotal = tariffDuties + zonosFees;
  const totalShipping = shippingSubtotal + dutiesSubtotal;

  return {
    inputs: {
      itemValueAUD,
      weightBand,
      brandName,
      countryOfOrigin,
      tariffRate,
      includeExtraCover,
      discountBand
    },
    breakdown: {
      ausPostShipping,
      extraCover,
      shippingSubtotal,
      tariffDuties,
      zonosFees,
      dutiesSubtotal
    },
    totalShipping: Math.round(totalShipping * 100) / 100,
    warnings: {
      extraCoverRecommended: shouldWarnExtraCover(itemValueAUD, includeExtraCover)
    }
  };
}

/**
 * Get weight band from weight in grams
 * @param {number} weightGrams - Weight in grams
 * @returns {string} - Weight band name
 */
function getWeightBandFromGrams(weightGrams) {
  if (weightGrams < 250) return 'XSmall';
  if (weightGrams < 500) return 'Small';
  if (weightGrams < 1000) return 'Medium';
  if (weightGrams < 1500) return 'Large';
  return 'XLarge';
}

/**
 * Get all available brands
 * @returns {string[]} - Array of brand names
 */
function getAvailableBrands() {
  return Object.keys(brands.brands).sort();
}

/**
 * Get all available weight bands
 * @returns {Object[]} - Array of weight band info
 */
function getWeightBands() {
  const zone = postalRates.zones['3-USA & Canada'];
  return Object.entries(zone.weightBands).map(([key, data]) => ({
    key,
    label: data.label,
    maxWeight: data.maxWeight,
    basePrice: data.basePrice
  }));
}

/**
 * Get all countries with tariff rates
 * @returns {Object[]} - Array of country/rate pairs
 */
function getTariffCountries() {
  return Object.entries(tariffs.usaTariffs.rates).map(([country, rate]) => ({
    country,
    rate,
    ratePercent: Math.round(rate * 100)
  }));
}

module.exports = {
  calculateUSAShipping,
  calculateAusPostShipping,
  calculateExtraCover,
  calculateTariffDuties,
  calculateZonosFees,
  getCountryOfOrigin,
  getTariffRate,
  getWeightBandFromGrams,
  getAvailableBrands,
  getWeightBands,
  getTariffCountries,
  shouldWarnExtraCover
};
