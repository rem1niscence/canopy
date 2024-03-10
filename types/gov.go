package types

type ParamSpace interface {
	SetString(address string, paramName string, value string) ErrorI
	SetUint64(address string, paramName string, value uint64) ErrorI
	SetOwner(paramName string, owner string) ErrorI
}
