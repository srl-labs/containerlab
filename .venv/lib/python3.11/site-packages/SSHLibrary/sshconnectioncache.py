from robot.utils import ConnectionCache


class SSHConnectionCache(ConnectionCache):
    def __init__(self):
        ConnectionCache.__init__(self, no_current_msg='No open connection.')

    @property
    def connections(self):
        return self._connections

    @property
    def aliases(self):
        return self._aliases

    def close_current(self):
        connection = self.current
        connection.close()
        if connection.config.alias is not None:
            self.aliases.pop(connection.config.alias)
        idx = connection.config.index - 1
        self.connections[idx] = self.current = self._no_current

    def close_all(self):
        open_connections = (conn for conn in self._connections if conn)
        for connection in open_connections:
            connection.close()
        self.empty_cache()
        return self.current

    def get_connection(self, alias_or_index=None):
        connection = super(SSHConnectionCache, self).get_connection(alias_or_index)
        if not connection:
            raise RuntimeError("Non-existing index or alias '%s'." % alias_or_index)
        return connection
