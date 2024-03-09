package types

type ParamSpace interface {
	SetString(paramName string, value string) ErrorI
	SetUint64(paramName string, value uint64) ErrorI
	SetOwner(paramName string, owner string) ErrorI
}
