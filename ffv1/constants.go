package ffv1

// Internal constants.
const (
	maxQuantTables   = 8  // Only defined in FFmpeg?
	maxContextInputs = 5  // 4.9. Quantization Table Set
	contextSize      = 32 // 4.1. Parameters
)

// API constants.

// Colorspaces.
// From 4.1.5. colorspace_type
const (
	YCbCr = 0
	RGB   = 1
)
