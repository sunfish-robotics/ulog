#!/usr/bin/env python3
"""Black-box pyulog oracle used by Go interoperability tests."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import numpy as np
from pyulog import ULog


def json_value(value: Any) -> Any:
    if isinstance(value, np.ndarray):
        return [json_value(item) for item in value.tolist()]
    if isinstance(value, np.generic):
        return value.item()
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    if isinstance(value, dict):
        return {str(key): json_value(item) for key, item in sorted(value.items())}
    if isinstance(value, (list, tuple)):
        return [json_value(item) for item in value]
    return value


def inspect_log(source: Path) -> dict[str, Any]:
    log = ULog(str(source))
    datasets = []
    for dataset in sorted(log.data_list, key=lambda item: (item.name, item.multi_id)):
        fields = []
        for name, values in sorted(dataset.data.items()):
            fields.append(
                {
                    "name": name,
                    "dtype": str(values.dtype),
                    "values": json_value(values),
                }
            )
        datasets.append(
            {
                "name": dataset.name,
                "multi_id": dataset.multi_id,
                "fields": fields,
            }
        )
    return {
        "start_timestamp": log.start_timestamp,
        "information": json_value(log.msg_info_dict),
        "initial_parameters": json_value(log.initial_parameters),
        "logs": [
            {
                "level": item.log_level,
                "timestamp": item.timestamp,
                "message": item.message,
            }
            for item in log.logged_messages
        ],
        "dropouts": [
            {"duration": item.duration, "timestamp": item.timestamp}
            for item in log.dropouts
        ],
        "datasets": datasets,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    subcommands = parser.add_subparsers(dest="command", required=True)

    inspect_parser = subcommands.add_parser("inspect")
    inspect_parser.add_argument("source", type=Path)

    rewrite_parser = subcommands.add_parser("rewrite")
    rewrite_parser.add_argument("source", type=Path)
    rewrite_parser.add_argument("destination", type=Path)

    args = parser.parse_args()
    if args.command == "inspect":
        print(json.dumps(inspect_log(args.source), sort_keys=True, separators=(",", ":")))
        return

    log = ULog(str(args.source))
    log.write_ulog(str(args.destination))


if __name__ == "__main__":
    main()
