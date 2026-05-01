"""Conflux CLI entry point."""

import click
from .engine import Engine


@click.group()
@click.option(
    "--config",
    "config_path",
    type=click.Path(exists=True),
    help="Path to config.yaml (default: ./config.yaml)",
)
@click.pass_context
def cli(ctx, config_path):
    ctx.ensure_object(dict)
    ctx.obj["config_path"] = config_path or "./config.yaml"


@cli.command()
@click.pass_context
def sync(ctx):
    """Start continuous session sync."""
    engine = Engine(ctx.obj["config_path"])
    engine.run()


@cli.command()
@click.pass_context
def once(ctx):
    """Run a single sync pass then exit."""
    engine = Engine(ctx.obj["config_path"])
    engine.run_once()


def main():
    cli()


if __name__ == "__main__":
    main()
