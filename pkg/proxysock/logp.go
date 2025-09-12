package proxysock

import "log"

type PrintLog struct {
	Prefix, Suffix string
}

func (pl *PrintLog) Print(mes ...any) {
	mes = append([]any{pl.Prefix}, mes...)
	mes = append(mes, pl.Suffix)
	log.Println(mes...)
}

func (pl *PrintLog) PrintF(format string, mes ...any) {
	mes = append([]any{pl.Prefix}, mes...)
	mes = append(mes, pl.Suffix)
	log.Printf("%s "+format+" %s", mes...)
}

func NewPrintLog(prifix, suffix string) *PrintLog {
	return &PrintLog{Prefix: prifix, Suffix: suffix}
}
