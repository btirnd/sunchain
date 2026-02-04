import json
from typing import Any, Dict, Iterable, List, Optional, Tuple

import psycopg2
from psycopg2.extras import execute_batch


class PostgreSQLAdapter:
    """PostgreSQL adapter for persisting blocks, transactions, and state."""

    def __init__(self, dsn: str) -> None:
        self._dsn = dsn

    def connect(self):
        return psycopg2.connect(self._dsn)

    def init_schema(self) -> None:
        with self.connect() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS engine_state (
                    block_height BIGINT PRIMARY KEY,
                    state_json JSONB NOT NULL,
                    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
                );
                """
            )
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS engine_transactions (
                    block_height BIGINT NOT NULL,
                    tx_index INT NOT NULL,
                    tx_json JSONB NOT NULL,
                    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                    PRIMARY KEY (block_height, tx_index),
                    FOREIGN KEY (block_height)
                        REFERENCES engine_state (block_height)
                        ON DELETE CASCADE
                );
                """
            )

    def save_block(
        self, block_height: int, transactions: Iterable[Dict[str, Any]], state: Dict[str, Any]
    ) -> None:
        state_payload = json.dumps(state)
        transactions_payload: List[Tuple[int, int, str]] = [
            (block_height, index, json.dumps(tx))
            for index, tx in enumerate(transactions)
        ]
        with self.connect() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                INSERT INTO engine_state (block_height, state_json)
                VALUES (%s, %s)
                ON CONFLICT (block_height)
                DO UPDATE SET state_json = EXCLUDED.state_json;
                """,
                (block_height, state_payload),
            )
            if transactions_payload:
                execute_batch(
                    cursor,
                    """
                    INSERT INTO engine_transactions (block_height, tx_index, tx_json)
                    VALUES (%s, %s, %s)
                    ON CONFLICT (block_height, tx_index)
                    DO UPDATE SET tx_json = EXCLUDED.tx_json;
                    """,
                    transactions_payload,
                )

    def load_latest_state(self) -> Tuple[int, Dict[str, Any]]:
        with self.connect() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT block_height, state_json
                FROM engine_state
                ORDER BY block_height DESC
                LIMIT 1;
                """
            )
            row = cursor.fetchone()
            if row is None:
                return 0, {}
            block_height, state_json = row
            return int(block_height), state_json

    def load_transactions(self, block_height: int) -> List[Dict[str, Any]]:
        with self.connect() as connection, connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT tx_json
                FROM engine_transactions
                WHERE block_height = %s
                ORDER BY tx_index ASC;
                """,
                (block_height,),
            )
            return [row[0] for row in cursor.fetchall()]
