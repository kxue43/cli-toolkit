# Builtin
from __future__ import annotations
from getpass import getpass
import json
from typing import TYPE_CHECKING

# External
import click

# Own
from ..aws import CredentialProcessOutput, save_cache, get_active_from_cache

# Type Checking
if TYPE_CHECKING:
    from mypy_boto3_sts import STSClient


@click.group()
def cli():
    pass


@cli.command()
@click.option("--role-arn", required=True, help="ARN of the IAM role to assume")
@click.option(
    "--mfa-serial",
    required=True,
    help="ARN of the virtual MFA to use when assuming the role",
)
@click.option("--profile", required=True, help="Source profile name")
@click.option("--role-session-name", default="ToolkitCLI", help="Role session name")
@click.option(
    "--duration-seconds",
    type=int,
    default=3600,
    help="Role assumption duration seconds",
)
def run_credential_process(
    role_arn: str,
    mfa_serial: str,
    profile: str,
    role_session_name: str,
    duration_seconds: int,
) -> None:
    output = get_active_from_cache(role_arn)
    if output is not None:
        click.echo(output)
        return

    import boto3

    client: STSClient = boto3.session.Session(profile_name=profile).client("sts")
    token_code = getpass("MFA code: ")
    resp = client.assume_role(
        RoleArn=role_arn,
        RoleSessionName=role_session_name,
        DurationSeconds=duration_seconds,
        SerialNumber=mfa_serial,
        TokenCode=token_code,
    )
    creds = resp["Credentials"]
    data: CredentialProcessOutput = {
        "Version": 1,
        "AccessKeyId": creds["AccessKeyId"],
        "SecretAccessKey": creds["SecretAccessKey"],
        "SessionToken": creds["SessionToken"],
        "Expiration": creds["Expiration"].isoformat(),
    }
    save_cache(role_arn, data)
    click.echo(json.dumps(data))
