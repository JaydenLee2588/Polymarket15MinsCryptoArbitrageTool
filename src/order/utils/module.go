package utils

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"Polymarket15MinsCryptoArbitrageTool/src/order/config"
	"Polymarket15MinsCryptoArbitrageTool/src/order/model"
)

func GetVerifyingContractAddress(chainId *big.Int, contract model.VerifyingContract) (common.Address, error) {
	contracts, err := config.GetContracts(chainId.Int64())
	if err != nil {
		return common.Address{}, err
	}

	switch contract {
	case model.CTFExchange:
		return contracts.Exchange, nil
	case model.NegRiskCTFExchange:
		return contracts.NegRiskExchange, nil
	}

	return common.Address{}, fmt.Errorf("invalid contract")
}
