class PluginartError(Exception):
    """Base error for pluginart runtime failures."""


class ConfigError(PluginartError):
    """Invalid pluginart.toml."""


class HandshakeError(PluginartError):
    """Protocol handshake failed."""


class PluginError(PluginartError):
    """Plugin returned an application error."""

    def __init__(self, message: str, retry: bool = False) -> None:
        super().__init__(message)
        self.retry = retry


class TransportError(PluginartError):
    """Socket transport failed."""


class StartupError(PluginartError):
    """Plugin process or container failed to start."""
