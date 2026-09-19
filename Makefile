# Makefile for Hatvan OS (Motorola 6809 / Hitachi 6309 and Motorola 68000)

REPO_DIR    := $(CURDIR)
BUILD_DIR   := $(REPO_DIR)/build

# Toolchains
GO          ?= go
PYTHON      ?= python3
MINIGOLF    ?= /home/strick/github.com/strickyak/minigolf/minigolf
ASM68K      ?= /home/strick/github.com/strickyak/minigolf/asm68k
LWASM       ?= lwasm
OS9         ?= os9
SREC2DECB   := $(REPO_DIR)/scripts/srec2decb.py

# VM Targets
VM_6809     := $(BUILD_DIR)/gep9
VM_68K      := $(BUILD_DIR)/gepk

# Kernel Targets
KERNEL_6809 := $(BUILD_DIR)/kernel_6809.decb
KERNEL_68K  := $(BUILD_DIR)/kernel_68k.srec

# Command Targets
CMDS_6809   := $(BUILD_DIR)/echo.mod $(BUILD_DIR)/testcmd.mod $(BUILD_DIR)/testdecb.decb
CMDS_68K    := $(BUILD_DIR)/echok.decb

# Disk Images
DISK_IMAGE  := $(BUILD_DIR)/disk0.dsk
TEST_DISK   := $(BUILD_DIR)/test.dsk

# Source Dependencies
COMMON_SRCS := $(wildcard $(REPO_DIR)/kernel/common/*.golf)
KLIB_SRCS   := $(wildcard $(REPO_DIR)/kernel/klib/*.golf)
M6809_SRCS  := $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm
M68K_SRCS   := $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(REPO_DIR)/kernel/m68k/trap_m68k.s

.PHONY: all vms kernels cmds disk test test-interactive clean

all: $(BUILD_DIR) vms kernels cmds disk

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# --- Toolchain Binaries ---
$(MINIGOLF):
	@mkdir -p $(dir $@)
	cd /home/strick/github.com/strickyak/minigolf && $(GO) build -o $(MINIGOLF) .

$(ASM68K):
	@mkdir -p $(dir $@)
	cd /home/strick/github.com/strickyak/minigolf && $(GO) build -o $(ASM68K) ./asm68k

# --- Emulators ---
vms: $(VM_6809) $(VM_68K)

$(VM_6809): $(shell find $(REPO_DIR)/cmd/gep9 $(REPO_DIR)/vm -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep9

$(VM_68K): $(shell find $(REPO_DIR)/cmd/gepk $(REPO_DIR)/vmk -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepk

# --- Kernels ---
kernels: $(KERNEL_6809) $(KERNEL_68K)

# M6809 Kernel
$(BUILD_DIR)/kernel_6809.asm: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m M6809 \
		-I $(REPO_DIR)/kernel/m6809 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_6809.asm: $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(BUILD_DIR)/kernel_6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(BUILD_DIR)/kernel_6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm > $@

$(BUILD_DIR)/kernel_6809.decb: $(BUILD_DIR)/full_6809.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --decb --list=kernel_6809.list --map=kernel_6809.map -o kernel_6809.decb full_6809.asm
	cp -f $(BUILD_DIR)/kernel_6809.decb.list $(BUILD_DIR)/kernel_6809.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/kernel_6809.decb.map $(BUILD_DIR)/kernel_6809.map 2>/dev/null || true

# M68K Kernel
$(BUILD_DIR)/kernel_68k.s: $(COMMON_SRCS) $(KLIB_SRCS) $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=k \
		-I $(REPO_DIR)/kernel/m68k \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$(REPO_DIR)/kernel/common/main.golf

$(BUILD_DIR)/full_68k.s: $(REPO_DIR)/kernel/m68k/trap_m68k.s $(BUILD_DIR)/kernel_68k.s | $(BUILD_DIR)
	cat $(REPO_DIR)/kernel/m68k/trap_m68k.s $(BUILD_DIR)/kernel_68k.s > $@

$(BUILD_DIR)/kernel_68k.srec: $(BUILD_DIR)/full_68k.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -o $@ $<

# --- Userland Commands ---
cmds: $(CMDS_6809) $(CMDS_68K)

$(BUILD_DIR)/echo.mod: $(REPO_DIR)/cmds/echo.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=echo.list --map=echo.map -o echo.mod $<
	cp -f $(BUILD_DIR)/echo.mod.list $(BUILD_DIR)/echo.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/echo.mod.map $(BUILD_DIR)/echo.map 2>/dev/null || true

$(BUILD_DIR)/testcmd.mod: $(REPO_DIR)/cmds/testcmd.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=testcmd.list --map=testcmd.map -o testcmd.mod $<
	cp -f $(BUILD_DIR)/testcmd.mod.list $(BUILD_DIR)/testcmd.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/testcmd.mod.map $(BUILD_DIR)/testcmd.map 2>/dev/null || true

$(BUILD_DIR)/testdecb.decb: $(REPO_DIR)/cmds/testdecb.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --decb --list=testdecb.list --map=testdecb.map -o testdecb.decb $<
	cp -f $(BUILD_DIR)/testdecb.decb.list $(BUILD_DIR)/testdecb.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/testdecb.decb.map $(BUILD_DIR)/testdecb.map 2>/dev/null || true

$(BUILD_DIR)/echok.srec: $(REPO_DIR)/cmds/echok.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -o $@ $<

$(BUILD_DIR)/echok.decb: $(BUILD_DIR)/echok.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

# --- OS-9 Disk Images ---
disk: $(DISK_IMAGE) $(TEST_DISK)

$(DISK_IMAGE): $(BUILD_DIR)/echo.mod $(BUILD_DIR)/testcmd.mod $(BUILD_DIR)/testdecb.decb $(BUILD_DIR)/echok.decb | $(BUILD_DIR)
	rm -f $@ $(TEST_DISK)
	$(OS9) format -e -n'HATVAN' -l'40000' $@
	$(OS9) makdir $@,CMDS
	$(OS9) copy -r -l $(BUILD_DIR)/echo.mod $@,CMDS/ECHO
	$(OS9) copy -r -l $(BUILD_DIR)/echok.decb $@,CMDS/ECHOK
	$(OS9) copy -r -l $(BUILD_DIR)/testcmd.mod $@,CMDS/TESTCMD
	$(OS9) copy -r -l $(BUILD_DIR)/testdecb.decb $@,CMDS/TESTDECB
	cp -f $@ $(TEST_DISK)

$(TEST_DISK): $(DISK_IMAGE)

# --- Testing ---
test: all
	$(VM_6809) --disk0=$(DISK_IMAGE) $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) $(KERNEL_68K)

test-interactive: all
	$(VM_6809) --disk0=$(DISK_IMAGE) --input="help\npwd\npwx\nECHO hello from 6809\nexit\n" $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) -input="help\npwd\npwx\nECHOK hello from 68k\nexit\n" $(KERNEL_68K)

# --- Clean ---
clean:
	@mkdir -p $(BUILD_DIR)
	find $(BUILD_DIR) -mindepth 1 -delete 2>/dev/null || rm -rf $(BUILD_DIR)/*
