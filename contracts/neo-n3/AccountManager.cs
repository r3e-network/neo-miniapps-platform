using System;
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;

namespace ServiceLayer.Contracts
{
    // AccountManager tracks workspaces/accounts and linked wallets.
    public class AccountManager : SmartContract
    {
        private static readonly StorageMap Accounts = new(Storage.CurrentContext, "acct:");
        private static readonly StorageMap Wallets = new(Storage.CurrentContext, "wallet:");

        public static event Action<ByteString, UInt160> AccountCreated;
        public static event Action<ByteString, UInt160> WalletLinked;

        public struct Account
        {
            public ByteString Id;
            public UInt160 Owner;
            public ByteString MetadataHash;
        }

        public struct Wallet
        {
            public ByteString AccountId;
            public UInt160 Address;
            public byte Status; // 0=active,1=revoked
        }

        public static void Create(ByteString id, UInt160 owner, ByteString metadataHash)
        {
            if (id is null || id.Length == 0) throw new Exception("missing id");
            if (Accounts.Get(id) is not null) throw new Exception("exists");
            var acct = new Account
            {
                Id = id,
                Owner = owner,
                MetadataHash = metadataHash
            };
            Accounts.Put(id, StdLib.Serialize(acct));
            AccountCreated(id, owner);
        }

        public static Account Get(ByteString id)
        {
            var data = Accounts.Get(id);
            if (data is null || data.Length == 0) throw new Exception("not found");
            return (Account)StdLib.Deserialize(data);
        }

        public static void LinkWallet(ByteString accountId, UInt160 address)
        {
            var acct = Get(accountId);
            RequireOwner(acct.Owner);
            var key = accountId + address;
            var w = new Wallet
            {
                AccountId = accountId,
                Address = address,
                Status = 0
            };
            Wallets.Put(key, StdLib.Serialize(w));
            WalletLinked(accountId, address);
        }

        public static Wallet GetWallet(ByteString accountId, UInt160 address)
        {
            var key = accountId + address;
            var data = Wallets.Get(key);
            if (data is null || data.Length == 0) throw new Exception("not found");
            return (Wallet)StdLib.Deserialize(data);
        }

        private static void RequireOwner(UInt160 owner)
        {
            if (owner is null || !owner.IsValid || !Runtime.CheckWitness(owner))
            {
                throw new Exception("owner required");
            }
        }
    }
}
