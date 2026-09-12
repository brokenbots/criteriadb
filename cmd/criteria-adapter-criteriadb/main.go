package main

import (
	"github.com/brokenbots/criteria-go-adapter-sdk/adapterhost"
	"github.com/brokenbots/criteriadb/pkg/adapter"
)

func main() {
	criteriadbAdapter := adapter.NewCriteriaDBAdapter(nil)
	adapterhost.Serve(criteriadbAdapter)
}
