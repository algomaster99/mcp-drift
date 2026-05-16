#!/usr/bin/env python3
import argparse
from pathlib import Path


LIST_COLUMNS = {
    "tools": 1,
    "prompts": 2,
    "resources": 3,
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Increment README monitored-server drift counts."
    )
    parser.add_argument("readme", type=Path)
    parser.add_argument("server_cell")
    parser.add_argument("lists", nargs="+", choices=sorted(LIST_COLUMNS))
    return parser.parse_args()


def split_row(line: str) -> list[str]:
    return [cell.strip() for cell in line.strip().strip("|").split("|")]


def format_row(cells: list[str]) -> str:
    return "| " + " | ".join(cells) + " |"


def bump_counts(readme: Path, server_cell: str, changed_lists: list[str]) -> None:
    lines = readme.read_text().splitlines()
    updated = False

    for index, line in enumerate(lines):
        cells = split_row(line)
        if len(cells) != 4 or cells[0] != server_cell:
            continue

        for changed_list in changed_lists:
            column = LIST_COLUMNS[changed_list]
            if cells[column] in ("?", "✗"):
                # Auth-gated (?) or not supported (✗) — leave marker, don't count
                continue
            try:
                cells[column] = str(int(cells[column]) + 1)
            except ValueError as exc:
                raise SystemExit(
                    f"README count is not an integer for {server_cell} {changed_list}: {cells[column]}"
                ) from exc

        lines[index] = format_row(cells)
        updated = True
        break

    if not updated:
        raise SystemExit(f"server row not found in README: {server_cell}")

    readme.write_text("\n".join(lines) + "\n")


def main() -> None:
    args = parse_args()
    bump_counts(args.readme, args.server_cell, args.lists)


if __name__ == "__main__":
    main()
