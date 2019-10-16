package ffv1

// Internal constants.
const (
	maxQuantTables   = 8 // Only defined in FFmpeg?
	maxContextInputs = 5
	contextSize      = 32
)

// API constants.

// Colorspaces.
// From 4.1.5. colorspace_type
const (
	YCbCr = 0
	RGB   = 1
)
