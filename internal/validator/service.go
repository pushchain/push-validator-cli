package validator

import "context"

// KeyInfo contains structured information about a created/existing key
type KeyInfo struct {
    Address  string // Cosmos address (push1...)
    Name     string // Key name
    Pubkey   string // Public key JSON
    Type     string // Key type (local, ledger, etc)
    Mnemonic string // Recovery mnemonic phrase (only set on creation)
}

// Service handles key ops, balances, validator detection, and registration flow.
type Service interface {
    EnsureKey(ctx context.Context, name string) (KeyInfo, error)                  // returns key info
    ImportKey(ctx context.Context, name string, mnemonic string) (KeyInfo, error) // imports key from mnemonic
    GetEVMAddress(ctx context.Context, addr string) (string, error)               // returns hex/EVM address
    IsValidator(ctx context.Context, addr string) (bool, error)
    IsAddressValidator(ctx context.Context, cosmosAddr string) (bool, error) // checks if address controls a validator
    Balance(ctx context.Context, addr string) (string, error) // denom string for now
    Register(ctx context.Context, args RegisterArgs) (string, error) // returns tx hash
    Unjail(ctx context.Context, keyName string) (string, error) // returns tx hash
    WithdrawRewards(ctx context.Context, validatorAddr string, keyName string, includeCommission bool) (string, error) // returns tx hash
    Delegate(ctx context.Context, args DelegateArgs) (string, error) // returns tx hash
    Vote(ctx context.Context, args VoteArgs) (string, error) // returns tx hash
}

type RegisterArgs struct {
    Moniker string
    CommissionRate string
    MinSelfDelegation string
    Amount string
    KeyName string
}

type DelegateArgs struct {
    ValidatorAddress string
    Amount string
    KeyName string
}

type VoteArgs struct {
    ProposalID string
    Option     string // yes, no, abstain, no_with_veto
    KeyName    string
}

