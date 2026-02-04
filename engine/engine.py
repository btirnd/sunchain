from __future__ import annotations

from typing import Any, Dict, Iterable, List

from .db import PostgreSQLAdapter


class Engine:
    def __init__(self, adapter: PostgreSQLAdapter) -> None:
        self.adapter = adapter
        self.state: Dict[str, Any] = {}
        self.block_height = 0

    def recover(self) -> None:
        """Load the latest finalized state from the database."""
        self.adapter.init_schema()
        self.block_height, self.state = self.adapter.load_latest_state()

    def finalize_block(self, transactions: Iterable[Dict[str, Any]]) -> None:
        """Apply transactions, advance the height, and persist results."""
        new_state = self.apply_transactions(transactions)
        self.block_height += 1
        self.state = new_state
        self.adapter.save_block(self.block_height, transactions, self.state)

    def apply_transactions(self, transactions: Iterable[Dict[str, Any]]) -> Dict[str, Any]:
        updated_state = dict(self.state)
        for tx in transactions:
            self.apply_transaction(updated_state, tx)
        return updated_state

    @staticmethod
    def apply_transaction(state: Dict[str, Any], tx: Dict[str, Any]) -> None:
        """Apply a single transaction to the state.

        Expected transaction format:
        {"key": "account", "value": 10}
        """
        if "key" not in tx:
            raise ValueError("Transaction missing 'key'.")
        state[tx["key"]] = tx.get("value")

    def get_transactions_for_block(self, block_height: int) -> List[Dict[str, Any]]:
        return self.adapter.load_transactions(block_height)
