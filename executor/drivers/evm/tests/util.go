package tests

import (
	"encoding/hex"
	"fmt"

	"gitlab.33.cn/chain33/chain33/account"
	"gitlab.33.cn/chain33/chain33/common/crypto"
	"gitlab.33.cn/chain33/chain33/common/db"
	"gitlab.33.cn/chain33/chain33/executor/drivers/evm"
	"gitlab.33.cn/chain33/chain33/executor/drivers/evm/vm/common"
	crypto2 "gitlab.33.cn/chain33/chain33/executor/drivers/evm/vm/common/crypto"
	"gitlab.33.cn/chain33/chain33/executor/drivers/evm/vm/model"
	"gitlab.33.cn/chain33/chain33/executor/drivers/evm/vm/runtime"
	"gitlab.33.cn/chain33/chain33/executor/drivers/evm/vm/state"
	"gitlab.33.cn/chain33/chain33/types"
)

func getPrivKey() crypto.PrivKey {
	c, err := crypto.New(types.GetSignatureTypeName(types.SECP256K1))
	if err != nil {
		return nil
	}
	key, err := c.GenKey()
	if err != nil {
		return nil
	}
	return key
}

func getAddr(privKey crypto.PrivKey) *account.Address {
	return account.PubKeyToAddress(privKey.PubKey().Bytes())
}

func createTx(privKey crypto.PrivKey, code []byte, fee uint64, amount uint64) types.Transaction {

	action := types.EVMContractAction{Amount: amount, Code: code}
	tx := types.Transaction{Execer: []byte("user.evm"), Payload: types.Encode(&action), Fee: int64(fee)}
	tx.Sign(types.SECP256K1, privKey)
	return tx
}

func addAccount(mdb *db.GoMemDB, acc1 *types.Account) {
	acc := account.NewCoinsAccount()
	set := acc.GetKVSet(acc1)
	for i := 0; i < len(set); i++ {
		mdb.Set(set[i].GetKey(), set[i].Value)
	}
}

func addContractAccount(db *state.MemoryStateDB, mdb *db.GoMemDB, addr string, a AccountJson) {
	acc := state.NewContractAccount(addr, db)
	code, err := hex.DecodeString(a.code)
	if err != nil {
		fmt.Println(err)
	}
	acc.SetCode(code)
	acc.SetNonce(uint64(a.nonce))
	for k, v := range a.storage {
		key, _ := hex.DecodeString(k)
		value, _ := hex.DecodeString(v)
		acc.SetState(common.BytesToHash(key), common.BytesToHash(value))
	}
	set := acc.GetDataKV()
	set = append(set, acc.GetStateKV()...)
	for i := 0; i < len(set); i++ {
		mdb.Set(set[i].GetKey(), set[i].Value)
	}
}

func buildStateDB(addr string, balance int64) *db.GoMemDB {
	// 替换statedb中的数据库，获取测试需要的数据
	mdb, _ := db.NewGoMemDB("test", "", 0)

	// 将调用者账户设置进去，并给予金额，方便发起合约调用
	ac := &types.Account{Addr: addr, Balance: balance}
	addAccount(mdb, ac)

	return mdb
}

func createContract(mdb *db.GoMemDB, tx types.Transaction, maxCodeSize int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error, statedb *state.MemoryStateDB) {
	inst := evm.NewEVMExecutor()

	msg, _ := inst.GetMessage(&tx)

	inst.SetEnv(10, 0, "", uint64(10))
	statedb = inst.GetMStateDB()

	statedb.StateDB = mdb

	statedb.CoinsAccount = account.NewCoinsAccount()
	statedb.CoinsAccount.SetDB(statedb.StateDB)

	vmcfg := inst.GetVMConfig()

	context := inst.NewEVMContext(msg)

	// 创建EVM运行时对象
	env := runtime.NewEVM(context, statedb, *vmcfg)
	if maxCodeSize != 0 {
		env.SetMaxCodeSize(maxCodeSize)
	}

	addr := *crypto2.RandomContractAddress()
	ret, _, leftGas, err := env.Create(runtime.AccountRef(msg.From()), addr, msg.Data(), msg.GasLimit(), fmt.Sprintf("%s%s", model.EvmPrefix, common.BytesToHash(tx.Hash()).Hex()), "")

	return ret, addr, leftGas, err, statedb
}
