// Protocol constants mirroring firmware/src/core/gp_protocol.h.
// layout_test.go verifies every value against protocol/layout.json, which is
// generated from the C header.

package proto

const (
	MaxAxes               = 8
	MaxButtons            = 64
	MaxLedPins            = 8
	MaxLeds               = 64
	MaxGroups             = 4
	RadialBins            = 32
	StrLen                = 32
	AdcMax                = 4095
	PinNone               = 255
	ProtoVersion          = 1
	ConfigMagic           = 1128747090
	ConfigVersion         = 1
	CalMagic              = 1094930514
	CalVersion            = 1
	AxisInvert            = 1
	AxisLinear            = 2
	AxisCustomDz          = 4
	UsageX                = 48
	UsageY                = 49
	UsageZ                = 50
	UsageRx               = 51
	UsageRy               = 52
	UsageRz               = 53
	UsageSlider           = 54
	UsageDial             = 55
	GroupSwap             = 1
	GroupInvertH          = 2
	GroupInvertV          = 4
	GroupCustomDz         = 8
	ShapeCircle           = 0
	ShapeSquare           = 1
	LedDither             = 1
	LedKeepOnSleep        = 2
	LedFxGlow             = 0
	LedFxBar              = 1
	LedFxDot              = 2
	SmoothSimple          = 0
	SmoothLinear          = 1
	SmoothTriangle        = 2
	HidGamepad            = 0
	HidJoystick           = 1
	OrientSwap            = 1
	OrientInvertH         = 2
	OrientInvertV         = 4
	FrameSync0            = 170
	FrameSync1            = 85
	FrameOverhead         = 9
	ProtoMaxPayload       = 2048
	CmdHello              = 1
	CmdGetLimits          = 2
	CmdReadConfig         = 3
	CmdValidateConfig     = 4
	CmdWriteConfig        = 5
	CmdReadCalibration    = 6
	CmdWriteCalibration   = 7
	CmdTelemetry          = 8
	CmdCalStart           = 9
	CmdCalFinish          = 10
	CmdCalCancel          = 11
	CmdLedPreview         = 12
	CmdLedPreviewStop     = 13
	CmdFactoryReset       = 14
	CmdReboot             = 15
	CmdResetCalibration   = 16
	CmdSimInput           = 112
	StatusOk              = 0
	StatusInvalid         = 1
	StatusBadLength       = 2
	StatusBadState        = 3
	StatusStorage         = 4
	StatusUnknownCmd      = 5
	PreviewTimeoutMs      = 1500
	HelloSafeMode         = 1
	HelloConfigDefaulted  = 2
	HelloCalDefaulted     = 4
	HelloStorageEmpty     = 8
	HelloStorageRecovered = 16
	HelloSimulator        = 32768
	ResetPowerOn          = 0
	ResetWatchdog         = 1
	ResetRequested        = 2
	FNone                 = 0
	FHeader               = 1
	FUsbVid               = 2
	FUsbPid               = 3
	FUsbManufacturer      = 4
	FUsbProduct           = 5
	FUsbSerial            = 6
	FHidRate              = 7
	FHidReportID          = 8
	FHidDeviceType        = 9
	FPollRate             = 10
	FSmoothingSamples     = 11
	FSmoothingMode        = 12
	FDebounce             = 13
	FCenterDz             = 14
	FOuterDz              = 15
	FNumAxes              = 16
	FNumButtons           = 17
	FNumLedPins           = 18
	FCalPin               = 19
	FCalHold              = 20
	FResetHold            = 21
	FBrightness           = 22
	FStatusBrightness     = 23
	FLedPin               = 24
	FStatusLedMask        = 25
	FLedFlags             = 26
	FAxisPin              = 27
	FAxisUsage            = 28
	FAxisFlags            = 29
	FAxisCenterDz         = 30
	FAxisOuterDz          = 31
	FAxisLedMask          = 32
	FAxisColor            = 33
	FButtonPin            = 34
	FButtonFlags          = 35
	FButtonLedMask        = 36
	FButtonColor          = 37
	FGroupEnabled         = 38
	FGroupAxes            = 39
	FGroupShape           = 40
	FGroupFlags           = 41
	FGroupCenterDz        = 42
	FGroupOuterDz         = 43
	FCalChannel           = 44
	FCalGroup             = 45
	FLedCount             = 46
	FAxisLedEffect        = 47
	FCount                = 48
	MaxErrors             = 16
	ErrorMsgLen           = 60
	StateCalibrating      = 1
	StateUsbMounted       = 2
	StateSleeping         = 4
	StatePreview          = 8
	StateSafeMode         = 16
	StateCalButton        = 32
	StateResetPending     = 64
	CalChIdle             = 0
	CalChMoving           = 1
	CalChOk               = 2
	CalGrpIdle            = 0
	CalGrpUp              = 1
	CalGrpRotating        = 2
	CalGrpDone            = 3
	CalResUnchanged       = 0
	CalResUpdated         = 1
	CalResFailed          = 2
)

// cConstants maps the C names to the Go constants (used by layout_test.go).
var cConstants = map[string]int64{
	"GP_MAX_AXES":                MaxAxes,
	"GP_MAX_BUTTONS":             MaxButtons,
	"GP_MAX_LED_PINS":            MaxLedPins,
	"GP_MAX_LEDS":                MaxLeds,
	"GP_MAX_GROUPS":              MaxGroups,
	"GP_RADIAL_BINS":             RadialBins,
	"GP_STR_LEN":                 StrLen,
	"GP_ADC_MAX":                 AdcMax,
	"GP_PIN_NONE":                PinNone,
	"GP_PROTO_VERSION":           ProtoVersion,
	"GP_CONFIG_MAGIC":            ConfigMagic,
	"GP_CONFIG_VERSION":          ConfigVersion,
	"GP_CAL_MAGIC":               CalMagic,
	"GP_CAL_VERSION":             CalVersion,
	"GP_AXIS_INVERT":             AxisInvert,
	"GP_AXIS_LINEAR":             AxisLinear,
	"GP_AXIS_CUSTOM_DZ":          AxisCustomDz,
	"GP_USAGE_X":                 UsageX,
	"GP_USAGE_Y":                 UsageY,
	"GP_USAGE_Z":                 UsageZ,
	"GP_USAGE_RX":                UsageRx,
	"GP_USAGE_RY":                UsageRy,
	"GP_USAGE_RZ":                UsageRz,
	"GP_USAGE_SLIDER":            UsageSlider,
	"GP_USAGE_DIAL":              UsageDial,
	"GP_GROUP_SWAP":              GroupSwap,
	"GP_GROUP_INVERT_H":          GroupInvertH,
	"GP_GROUP_INVERT_V":          GroupInvertV,
	"GP_GROUP_CUSTOM_DZ":         GroupCustomDz,
	"GP_SHAPE_CIRCLE":            ShapeCircle,
	"GP_SHAPE_SQUARE":            ShapeSquare,
	"GP_LED_DITHER":              LedDither,
	"GP_LED_KEEP_ON_SLEEP":       LedKeepOnSleep,
	"GP_LED_FX_GLOW":             LedFxGlow,
	"GP_LED_FX_BAR":              LedFxBar,
	"GP_LED_FX_DOT":              LedFxDot,
	"GP_SMOOTH_SIMPLE":           SmoothSimple,
	"GP_SMOOTH_LINEAR":           SmoothLinear,
	"GP_SMOOTH_TRIANGLE":         SmoothTriangle,
	"GP_HID_GAMEPAD":             HidGamepad,
	"GP_HID_JOYSTICK":            HidJoystick,
	"GP_ORIENT_SWAP":             OrientSwap,
	"GP_ORIENT_INVERT_H":         OrientInvertH,
	"GP_ORIENT_INVERT_V":         OrientInvertV,
	"GP_FRAME_SYNC0":             FrameSync0,
	"GP_FRAME_SYNC1":             FrameSync1,
	"GP_FRAME_OVERHEAD":          FrameOverhead,
	"GP_PROTO_MAX_PAYLOAD":       ProtoMaxPayload,
	"GP_CMD_HELLO":               CmdHello,
	"GP_CMD_GET_LIMITS":          CmdGetLimits,
	"GP_CMD_READ_CONFIG":         CmdReadConfig,
	"GP_CMD_VALIDATE_CONFIG":     CmdValidateConfig,
	"GP_CMD_WRITE_CONFIG":        CmdWriteConfig,
	"GP_CMD_READ_CALIBRATION":    CmdReadCalibration,
	"GP_CMD_WRITE_CALIBRATION":   CmdWriteCalibration,
	"GP_CMD_TELEMETRY":           CmdTelemetry,
	"GP_CMD_CAL_START":           CmdCalStart,
	"GP_CMD_CAL_FINISH":          CmdCalFinish,
	"GP_CMD_CAL_CANCEL":          CmdCalCancel,
	"GP_CMD_LED_PREVIEW":         CmdLedPreview,
	"GP_CMD_LED_PREVIEW_STOP":    CmdLedPreviewStop,
	"GP_CMD_FACTORY_RESET":       CmdFactoryReset,
	"GP_CMD_REBOOT":              CmdReboot,
	"GP_CMD_RESET_CALIBRATION":   CmdResetCalibration,
	"GP_CMD_SIM_INPUT":           CmdSimInput,
	"GP_STATUS_OK":               StatusOk,
	"GP_STATUS_INVALID":          StatusInvalid,
	"GP_STATUS_BAD_LENGTH":       StatusBadLength,
	"GP_STATUS_BAD_STATE":        StatusBadState,
	"GP_STATUS_STORAGE":          StatusStorage,
	"GP_STATUS_UNKNOWN_CMD":      StatusUnknownCmd,
	"GP_PREVIEW_TIMEOUT_MS":      PreviewTimeoutMs,
	"GP_HELLO_SAFE_MODE":         HelloSafeMode,
	"GP_HELLO_CONFIG_DEFAULTED":  HelloConfigDefaulted,
	"GP_HELLO_CAL_DEFAULTED":     HelloCalDefaulted,
	"GP_HELLO_STORAGE_EMPTY":     HelloStorageEmpty,
	"GP_HELLO_STORAGE_RECOVERED": HelloStorageRecovered,
	"GP_HELLO_SIMULATOR":         HelloSimulator,
	"GP_RESET_POWER_ON":          ResetPowerOn,
	"GP_RESET_WATCHDOG":          ResetWatchdog,
	"GP_RESET_REQUESTED":         ResetRequested,
	"GP_F_NONE":                  FNone,
	"GP_F_HEADER":                FHeader,
	"GP_F_USB_VID":               FUsbVid,
	"GP_F_USB_PID":               FUsbPid,
	"GP_F_USB_MANUFACTURER":      FUsbManufacturer,
	"GP_F_USB_PRODUCT":           FUsbProduct,
	"GP_F_USB_SERIAL":            FUsbSerial,
	"GP_F_HID_RATE":              FHidRate,
	"GP_F_HID_REPORT_ID":         FHidReportID,
	"GP_F_HID_DEVICE_TYPE":       FHidDeviceType,
	"GP_F_POLL_RATE":             FPollRate,
	"GP_F_SMOOTHING_SAMPLES":     FSmoothingSamples,
	"GP_F_SMOOTHING_MODE":        FSmoothingMode,
	"GP_F_DEBOUNCE":              FDebounce,
	"GP_F_CENTER_DZ":             FCenterDz,
	"GP_F_OUTER_DZ":              FOuterDz,
	"GP_F_NUM_AXES":              FNumAxes,
	"GP_F_NUM_BUTTONS":           FNumButtons,
	"GP_F_NUM_LED_PINS":          FNumLedPins,
	"GP_F_CAL_PIN":               FCalPin,
	"GP_F_CAL_HOLD":              FCalHold,
	"GP_F_RESET_HOLD":            FResetHold,
	"GP_F_BRIGHTNESS":            FBrightness,
	"GP_F_STATUS_BRIGHTNESS":     FStatusBrightness,
	"GP_F_LED_PIN":               FLedPin,
	"GP_F_STATUS_LED_MASK":       FStatusLedMask,
	"GP_F_LED_FLAGS":             FLedFlags,
	"GP_F_AXIS_PIN":              FAxisPin,
	"GP_F_AXIS_USAGE":            FAxisUsage,
	"GP_F_AXIS_FLAGS":            FAxisFlags,
	"GP_F_AXIS_CENTER_DZ":        FAxisCenterDz,
	"GP_F_AXIS_OUTER_DZ":         FAxisOuterDz,
	"GP_F_AXIS_LED_MASK":         FAxisLedMask,
	"GP_F_AXIS_COLOR":            FAxisColor,
	"GP_F_BUTTON_PIN":            FButtonPin,
	"GP_F_BUTTON_FLAGS":          FButtonFlags,
	"GP_F_BUTTON_LED_MASK":       FButtonLedMask,
	"GP_F_BUTTON_COLOR":          FButtonColor,
	"GP_F_GROUP_ENABLED":         FGroupEnabled,
	"GP_F_GROUP_AXES":            FGroupAxes,
	"GP_F_GROUP_SHAPE":           FGroupShape,
	"GP_F_GROUP_FLAGS":           FGroupFlags,
	"GP_F_GROUP_CENTER_DZ":       FGroupCenterDz,
	"GP_F_GROUP_OUTER_DZ":        FGroupOuterDz,
	"GP_F_CAL_CHANNEL":           FCalChannel,
	"GP_F_CAL_GROUP":             FCalGroup,
	"GP_F_LED_COUNT":             FLedCount,
	"GP_F_AXIS_LED_EFFECT":       FAxisLedEffect,
	"GP_F_COUNT":                 FCount,
	"GP_MAX_ERRORS":              MaxErrors,
	"GP_ERROR_MSG_LEN":           ErrorMsgLen,
	"GP_STATE_CALIBRATING":       StateCalibrating,
	"GP_STATE_USB_MOUNTED":       StateUsbMounted,
	"GP_STATE_SLEEPING":          StateSleeping,
	"GP_STATE_PREVIEW":           StatePreview,
	"GP_STATE_SAFE_MODE":         StateSafeMode,
	"GP_STATE_CAL_BUTTON":        StateCalButton,
	"GP_STATE_RESET_PENDING":     StateResetPending,
	"GP_CALCH_IDLE":              CalChIdle,
	"GP_CALCH_MOVING":            CalChMoving,
	"GP_CALCH_OK":                CalChOk,
	"GP_CALGRP_IDLE":             CalGrpIdle,
	"GP_CALGRP_UP":               CalGrpUp,
	"GP_CALGRP_ROTATING":         CalGrpRotating,
	"GP_CALGRP_DONE":             CalGrpDone,
	"GP_CALRES_UNCHANGED":        CalResUnchanged,
	"GP_CALRES_UPDATED":          CalResUpdated,
	"GP_CALRES_FAILED":           CalResFailed,
}
