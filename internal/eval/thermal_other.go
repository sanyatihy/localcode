//go:build !darwin

package eval

// sampleThermal answers nothing off macOS. Linux has no cumulative equivalent of pmset's
// thermal warning — what throttling it exposes is per-driver and per-sensor rather than one
// figure for the machine — so the sample is unknown here.
//
// Unknown and not 100: 100 is the reading a machine gives when it has never been warned, so
// returning it off macOS would make every run on the GB10 read as unthrottled, and the
// probe exists to say when a timing measures the cap instead of the model.
func sampleThermal() ThermalSample { return ThermalSample{} }
