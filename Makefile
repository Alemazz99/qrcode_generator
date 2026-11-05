# Il servizio dei QR dinamici sta in server/: questi target lo richiamano da qui.
run build test check clean:
	$(MAKE) -C server $@

.PHONY: run build test check clean
