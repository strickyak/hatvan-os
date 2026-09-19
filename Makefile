# Makefile for Hatvan OS (Motorola 6809 / Hitachi 6309 and Motorola 68000)

REPO_DIR    := $(CURDIR)
BUILD_DIR   := $(REPO_DIR)/build

# Toolchains
GO          ?= go
PYTHON      ?= python3
MINIGOLF_DIR ?= $(shell cd ../minigolf && pwd)
MINIGOLF    ?= $(BUILD_DIR)/minigolf
ASM68K      ?= $(BUILD_DIR)/asm68k
LWASM       ?= lwasm
OS9         ?= os9
SREC2DECB   := $(REPO_DIR)/scripts/srec2decb.py

# VM Targets
VM_6809     := $(BUILD_DIR)/gep9
VM_68K      := $(BUILD_DIR)/gepk

# Kernel Targets
KERNEL_6809 := $(BUILD_DIR)/kernel_6809.decb
KERNEL_68K  := $(BUILD_DIR)/kernel_68k.srec
RBF_6809    := $(BUILD_DIR)/rbf_6809.decb
RBF_68K     := $(BUILD_DIR)/rbf_68k.srec

# Command Targets
CMDS_6809   := $(BUILD_DIR)/echo.mod $(BUILD_DIR)/testcmd.mod $(BUILD_DIR)/testdecb.decb $(BUILD_DIR)/cat.mod
CMDS_68K    := $(BUILD_DIR)/echok.decb $(BUILD_DIR)/dirk.decb $(BUILD_DIR)/dumpk.decb $(BUILD_DIR)/catk.decb

# Disk Images
DISK_IMAGE  := $(BUILD_DIR)/disk0.dsk
TEST_DISK   := $(BUILD_DIR)/test.dsk

# Source Dependencies
COMMON_SRCS := $(wildcard $(REPO_DIR)/kernel/common/*.golf)
KLIB_SRCS   := $(wildcard $(REPO_DIR)/kernel/klib/*.golf)
M6809_SRCS  := $(wildcard $(REPO_DIR)/kernel/m6809/*.golf) $(REPO_DIR)/kernel/m6809/cstart_m6809.asm $(REPO_DIR)/kernel/m6809/trap_m6809.asm $(REPO_DIR)/kernel/m6809/vectors_m6809.asm
M68K_SRCS   := $(wildcard $(REPO_DIR)/kernel/m68k/*.golf) $(REPO_DIR)/kernel/m68k/trap_m68k.s

.PHONY: all vms kernels cmds disk test test-interactive clean

all: $(BUILD_DIR) vms kernels cmds disk test-interactive

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# --- Toolchain Binaries ---
$(MINIGOLF): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(MINIGOLF) .

$(ASM68K): | $(BUILD_DIR)
	cd $(MINIGOLF_DIR) && $(GO) build -o $(ASM68K) ./cmd/asm68k

# --- Emulators ---
vms: $(VM_6809) $(VM_68K)

$(VM_6809): $(shell find $(REPO_DIR)/cmd/gep9 $(REPO_DIR)/gep9 -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gep9

$(VM_68K): $(shell find $(REPO_DIR)/cmd/gepk $(REPO_DIR)/gepk -type f -name '*.go') | $(BUILD_DIR)
	$(GO) build -o $@ ./cmd/gepk

# --- Kernels & Drivers ---
kernels: $(KERNEL_6809) $(KERNEL_68K) $(RBF_6809) $(RBF_68K)

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

# M6809 RBF Driver (Task 1)
$(BUILD_DIR)/rbf_6809.asm: $(REPO_DIR)/drivers/rbf/main.golf $(REPO_DIR)/kernel/common/rbf.golf $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m M6809 \
		-I $(REPO_DIR)/kernel/m6809 \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_6809.asm: $(REPO_DIR)/drivers/rbf/cstart_rbf_m6809.asm $(BUILD_DIR)/rbf_6809.asm | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_m6809.asm $(BUILD_DIR)/rbf_6809.asm > $@

$(BUILD_DIR)/rbf_6809.decb: $(BUILD_DIR)/full_rbf_6809.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --decb --list=rbf_6809.list --map=rbf_6809.map -o rbf_6809.decb full_rbf_6809.asm
	cp -f $(BUILD_DIR)/rbf_6809.decb.list $(BUILD_DIR)/rbf_6809.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/rbf_6809.decb.map $(BUILD_DIR)/rbf_6809.map 2>/dev/null || true

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
	$(ASM68K) -l $@.list -o $@ $<

# M68K RBF Driver (Task 1)
$(BUILD_DIR)/rbf_68k.s: $(REPO_DIR)/drivers/rbf/main.golf $(REPO_DIR)/kernel/common/rbf.golf $(MINIGOLF) | $(BUILD_DIR)
	$(MINIGOLF) -m=k \
		-I $(REPO_DIR)/kernel/m68k \
		-I $(REPO_DIR)/kernel/common \
		-I $(REPO_DIR)/kernel/klib \
		-o $@ \
		$<

$(BUILD_DIR)/full_rbf_68k.s: $(REPO_DIR)/drivers/rbf/cstart_rbf_m68k.s $(BUILD_DIR)/rbf_68k.s | $(BUILD_DIR)
	cat $(REPO_DIR)/drivers/rbf/cstart_rbf_m68k.s $(BUILD_DIR)/rbf_68k.s > $@

$(BUILD_DIR)/rbf_68k.srec: $(BUILD_DIR)/full_rbf_68k.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

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
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/echok.decb: $(BUILD_DIR)/echok.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/dirk.srec: $(REPO_DIR)/cmds/dirk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/dirk.decb: $(BUILD_DIR)/dirk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/dumpk.srec: $(REPO_DIR)/cmds/dumpk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/dumpk.decb: $(BUILD_DIR)/dumpk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

$(BUILD_DIR)/cat.mod: $(REPO_DIR)/cmds/cat.asm | $(BUILD_DIR)
	cd $(BUILD_DIR) && $(LWASM) --format=os9 --list=cat.list --map=cat.map -o cat.mod $<
	cp -f $(BUILD_DIR)/cat.mod.list $(BUILD_DIR)/cat.list 2>/dev/null || true
	cp -f $(BUILD_DIR)/cat.mod.map $(BUILD_DIR)/cat.map 2>/dev/null || true

$(BUILD_DIR)/catk.srec: $(REPO_DIR)/cmds/catk.s $(ASM68K) | $(BUILD_DIR)
	$(ASM68K) -l $@.list -o $@ $<

$(BUILD_DIR)/catk.decb: $(BUILD_DIR)/catk.srec $(SREC2DECB) | $(BUILD_DIR)
	$(PYTHON) $(SREC2DECB) $< $@

# --- OS-9 Disk Images ---
disk: $(DISK_IMAGE) $(TEST_DISK)

$(DISK_IMAGE): $(CMDS_6809) $(CMDS_68K) $(REPO_DIR)/cmds/os9-6809-level1.zip | $(BUILD_DIR)
	rm -f $@ $(TEST_DISK)
	$(OS9) format -e -n'HATVAN' -l'40000' $@
	$(OS9) makdir $@,Cmds9
	$(OS9) copy -r -l $(BUILD_DIR)/echo.mod $@,Cmds9/ECHO
	$(OS9) copy -r -l $(BUILD_DIR)/cat.mod $@,Cmds9/CAT
	$(OS9) copy -r -l $(BUILD_DIR)/testcmd.mod $@,Cmds9/TESTCMD
	$(OS9) copy -r -l $(BUILD_DIR)/testdecb.decb $@,Cmds9/TESTDECB
	unzip -q -o $(REPO_DIR)/cmds/os9-6809-level1.zip -d $(BUILD_DIR)
	for f in $(BUILD_DIR)/os9-6809-level1/*; do $(OS9) copy -r "$$f" $@,Cmds9; done
	$(OS9) makdir $@,CmdsK
	$(OS9) copy -r -l $(BUILD_DIR)/echok.decb $@,CmdsK/ECHO
	$(OS9) copy -r -l $(BUILD_DIR)/dirk.decb $@,CmdsK/DIR
	$(OS9) copy -r -l $(BUILD_DIR)/dumpk.decb $@,CmdsK/DUMP
	$(OS9) copy -r -l $(BUILD_DIR)/catk.decb $@,CmdsK/CAT
	cp -f $@ $(TEST_DISK)

$(TEST_DISK): $(DISK_IMAGE)

# --- Testing ---
test: $(BUILD_DIR) vms kernels cmds disk
	$(VM_6809) --disk0=$(DISK_IMAGE) --input="exit\n" $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) -input="exit\n" $(KERNEL_68K)

test-interactive: $(BUILD_DIR) vms kernels cmds disk
	$(VM_6809) --disk0=$(DISK_IMAGE) --input="help\npwd\npwx\nECHO hello from 6809 userspace\nECHO redirection works on 6809 > /d0/redir9.txt\nCAT /d0/redir9.txt\nCAT /proc/p\nexit\n" $(KERNEL_6809)
	$(VM_68K) -disk0=$(DISK_IMAGE) -input="help\npwd\npwx\nECHO hello from 68k userspace\nECHO redirection works on 68k > /d0/redirk.txt\nCAT /d0/redirk.txt\nCAT /proc/p\nexit\n" $(KERNEL_68K)

# --- Clean ---
clean:
	@mkdir -p $(BUILD_DIR)
	find $(BUILD_DIR) -mindepth 1 -delete 2>/dev/null || rm -rf $(BUILD_DIR)/*
