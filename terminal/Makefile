ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)

$(or $(MAKECMDGOALS),all):
	@$(MAKE) --no-print-directory -C "$(ROOT_DIR)" $(MAKECMDGOALS)

.PHONY: $(MAKECMDGOALS)
