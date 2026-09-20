package data

import (
	"fmt"
	"strings"
)

type Call struct {
	Name                   string
	Desc                   string
	Number                 byte
	A, B, D, X, Y, U       string
	RA, RB, RD, RX, RY, RU string

	A_off, B_off, D_off, X_off, Y_off, U_off       int
	RA_off, RB_off, RD_off, RX_off, RY_off, RU_off int
}

var Calls = []*Call{
	// User and System State Calls ($00..$20)
	{
		Name:   "F$Link",
		Desc:   "Link to a memory module",
		Number: 0x00,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		RA:     "lang_and_type",
		RB:     "attr_and_rev",
		RX:     "after_module_name",
		RY:     "absolute_entry_addr",
		RU:     "absolute_header_addr",
	},
	{
		Name:   "F$Load",
		Desc:   "Load a module from a file",
		Number: 0x01,
		A:      "lang_and_type_code",
		X:      "module_name_ptr",
		RA:     "lang_and_type",
		RB:     "attr_and_rev",
		RX:     "after_module_name",
		RY:     "absolute_entry_addr",
		RU:     "absolute_header_addr",
	},
	{
		Name:   "F$UnLink",
		Desc:   "Unlink a memory module",
		Number: 0x02,
		U:      "module_hdr_ptr",
	},
	{
		Name:   "F$Fork",
		Desc:   "Create a child process",
		Number: 0x03,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		Y:      "size_of_param_area",
		U:      "start_of_param_area",
		RA:     "child_process_id",
		RX:     "after_module_name",
	},
	{
		Name:   "F$Wait",
		Desc:   "Wait for child process to die",
		Number: 0x04,
		RA:     "child_process_id",
		RB:     "child_exit_status",
	},
	{
		Name:   "F$Chain",
		Desc:   "Chain process to new module",
		Number: 0x05,
		A:      "lang_and_type",
		X:      "module_name_ptr",
		Y:      "size_of_param_area",
		U:      "start_of_param_area",
	},
	{
		Name:   "F$Exit",
		Desc:   "Terminate current process",
		Number: 0x06,
		B:      "status",
	},
	{
		Name:   "F$Mem",
		Desc:   "Set process memory size",
		Number: 0x07,
		D:      "num_bytes_wanted",
		RD:     "actual_num_bytes",
		RY:     "end_of_memory",
	},
	{
		Name:   "F$Send",
		Desc:   "Send signal to process",
		Number: 0x08,
		A:      "process_id",
		B:      "signal_code",
	},
	{
		Name:   "F$Icpt",
		Desc:   "Set signal intercept handler",
		Number: 0x09,
		X:      "func_address",
		U:      "start_memory_area",
	},
	{
		Name:   "F$Sleep",
		Desc:   "Suspend process for ticks",
		Number: 0x0A,
		X:      "num_ticks",
		RX:     "num_ticks_left",
	},
	{
		Name:   "F$SSpd",
		Desc:   "Suspend process execution",
		Number: 0x0B,
		A:      "process_id",
	},
	{
		Name:   "F$ID",
		Desc:   "Get process ID and user ID",
		Number: 0x0C,
		RA:     "process_id",
		RU:     "user_id",
	},
	{
		Name:   "F$SPrior",
		Desc:   "Set process priority",
		Number: 0x0D,
		A:      "process_id",
		B:      "priority",
	},
	{
		Name:   "F$SSWI",
		Desc:   "Set software interrupt vector",
		Number: 0x0E,
		A:      "swi_type",
		X:      "handler_address",
	},
	{
		Name:   "F$PErr",
		Desc:   "Print error message to stderr",
		Number: 0x0F,
		B:      "error_code",
	},
	{
		Name:   "F$PrsNam",
		Desc:   "Parse pathlist name",
		Number: 0x10,
		X:      "name",
		RA:     "trailing_byte",
		RB:     "name_length",
		RX:     "last_slash_plus1",
		RY:     "last_char_plus1",
	},
	{
		Name:   "F$CmpNam",
		Desc:   "Compare two names",
		Number: 0x11,
		X:      "name1_ptr",
		Y:      "name2_ptr",
		U:      "length",
	},
	{
		Name:   "F$SchBit",
		Desc:   "Search bit map",
		Number: 0x12,
		D:      "free_bit_count",
		X:      "bit_map_ptr",
		Y:      "bit_map_size",
		RU:     "bit_number",
	},
	{
		Name:   "F$AllBit",
		Desc:   "Allocate in bit map",
		Number: 0x13,
		D:      "num_bits",
		X:      "bit_map_ptr",
		U:      "bit_number",
	},
	{
		Name:   "F$DelBit",
		Desc:   "Deallocate in bit map",
		Number: 0x14,
		D:      "num_bits",
		X:      "bit_map_ptr",
		U:      "bit_number",
	},
	{
		Name:   "F$Time",
		Desc:   "Get current system time",
		Number: 0x15,
		X:      "time_buffer",
	},
	{
		Name:   "F$STime",
		Desc:   "Set current system time",
		Number: 0x16,
		X:      "time_buffer",
	},
	{
		Name:   "F$CRC",
		Desc:   "Generate CRC",
		Number: 0x17,
		X:      "start_address",
		Y:      "byte_count",
		U:      "crc_accumulator_ptr",
	},
	{
		Name:   "F$GPrDsc",
		Desc:   "Get process descriptor copy",
		Number: 0x18,
		A:      "process_id",
		X:      "buffer_512",
	},
	{
		Name:   "F$GBlkMp",
		Desc:   "Get system block map copy",
		Number: 0x19,
		X:      "buffer",
	},
	{
		Name:   "F$GModDr",
		Desc:   "Get module directory copy",
		Number: 0x1A,
		X:      "buffer",
		Y:      "buffer_size",
	},
	{
		Name:   "F$CpyMem",
		Desc:   "Copy memory between address spaces",
		Number: 0x1B,
		A:      "src_task",
		X:      "src_ptr",
		Y:      "byte_count",
		U:      "dest_ptr",
	},
	{
		Name:   "F$SUser",
		Desc:   "Set user ID",
		Number: 0x1C,
		U:      "user_id",
	},
	{
		Name:   "F$UnLoad",
		Desc:   "Unload module by name",
		Number: 0x1D,
		X:      "module_name_ptr",
	},
	{
		Name:   "F$Alarm",
		Desc:   "Set alarm clock",
		Number: 0x1E,
		X:      "alarm_packet_ptr",
	},
	{
		Name:   "F$SigBit",
		Desc:   "Signal process on bit change",
		Number: 0x1F,
		A:      "process_id",
		B:      "bit_number",
	},

	// System / Privileged Calls ($27..$50)
	{
		Name:   "F$VIRQ",
		Desc:   "Install/delete virtual IRQ",
		Number: 0x27,
		D:      "packet_addr",
		Y:      "service_routine",
	},
	{
		Name:   "F$SRqMem",
		Desc:   "System memory request",
		Number: 0x28,
		D:      "byte_count",
		RD:     "actual_size",
		RU:     "starting_addr",
	},
	{
		Name:   "F$SRtMem",
		Desc:   "System memory return",
		Number: 0x29,
		D:      "byte_count",
		U:      "starting_addr",
	},
	{
		Name:   "F$IRQ",
		Desc:   "Enter IRQ polling table",
		Number: 0x2A,
		D:      "polling_addr",
		X:      "packet_addr",
		Y:      "service_routine",
		U:      "memory_area",
	},
	{
		Name:   "F$IOQu",
		Desc:   "Enter I/O queue",
		Number: 0x2B,
		D:      "process_desc_addr",
	},
	{
		Name:   "F$AProc",
		Desc:   "Enter active process queue",
		Number: 0x2C,
		X:      "addr_of_process_desc",
	},
	{
		Name:   "F$NProc",
		Desc:   "Start next process",
		Number: 0x2D,
	},
	{
		Name:   "F$VModul",
		Desc:   "Validate module in memory",
		Number: 0x2E,
		D:      "DAT_image_ptr",
		X:      "new_module_block_offset",
		RU:     "module_directory_entry_addr",
	},
	{
		Name:   "F$Find64",
		Desc:   "Find 64-byte block / descriptor",
		Number: 0x2F,
		A:      "block_number",
		X:      "base_addr",
		RY:     "address_of_block",
	},
	{
		Name:   "F$All64",
		Desc:   "Allocate 64-byte block / descriptor",
		Number: 0x30,
		X:      "base_addr",
		RA:     "block_number",
		RX:     "base_addr_out",
		RY:     "address_of_block",
	},
	{
		Name:   "F$Ret64",
		Desc:   "Return 64-byte block / descriptor",
		Number: 0x31,
		A:      "block_number",
		X:      "base_addr",
	},
	{
		Name:   "F$SSvc",
		Desc:   "Service request table initialization",
		Number: 0x32,
		Y:      "init_table_addr",
	},
	{
		Name:   "F$IODel",
		Desc:   "Delete I/O module from system",
		Number: 0x33,
		X:      "module_hdr_ptr",
	},
	{
		Name:   "F$SLink",
		Desc:   "System link to module",
		Number: 0x34,
		A:      "module_type",
		X:      "module_name",
		Y:      "name_string_DAT_image_ptr",
	},
	{
		Name:   "F$Boot",
		Desc:   "Bootstrap system devices",
		Number: 0x35,
	},
	{
		Name:   "F$BtMod",
		Desc:   "Link to boot module",
		Number: 0x36,
	},
	{
		Name:   "F$GProcP",
		Desc:   "Get process descriptor pointer",
		Number: 0x37,
		A:      "proc_id",
		Y:      "proc_desc_addr",
	},
	{
		Name:   "F$Move",
		Desc:   "Move data across address spaces",
		Number: 0x38,
		A:      "src_task",
		B:      "dest_task",
		X:      "src_ptr",
		Y:      "byte_count",
		U:      "dest_ptr",
	},
	{
		Name:   "F$AllPrc",
		Desc:   "Allocate process descriptor",
		Number: 0x39,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$AllImg",
		Desc:   "Allocate RAM blocks for DAT image",
		Number: 0x3A,
		A:      "starting_block",
		B:      "num_blocks",
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$DelImg",
		Desc:   "Deallocate process DAT image",
		Number: 0x3B,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$SetImg",
		Desc:   "Set process DAT image",
		Number: 0x3C,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$FreeLB",
		Desc:   "Free low memory block",
		Number: 0x3D,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$FreeHB",
		Desc:   "Free high memory block",
		Number: 0x3E,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$AllTsk",
		Desc:   "Allocate process task number",
		Number: 0x3F,
		X:      "proc_desc_addr",
		RA:     "task_number",
	},
	{
		Name:   "F$DelTsk",
		Desc:   "Deallocate process task number",
		Number: 0x40,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$SetTsk",
		Desc:   "Set task DAT registers",
		Number: 0x41,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$ResTsk",
		Desc:   "Reserve task number",
		Number: 0x42,
		A:      "task_number",
	},
	{
		Name:   "F$RelTsk",
		Desc:   "Release task number",
		Number: 0x43,
		A:      "task_number",
	},
	{
		Name:   "F$DATRep",
		Desc:   "Replicate DAT image",
		Number: 0x44,
		X:      "src_proc_desc",
		U:      "dst_proc_desc",
	},
	{
		Name:   "F$DATTmp",
		Desc:   "Setup temporary DAT image",
		Number: 0x45,
		X:      "proc_desc_addr",
	},
	{
		Name:   "F$LDABX",
		Desc:   "Load A [D+X]",
		Number: 0x46,
		D:      "offset",
		X:      "base_addr",
		RA:     "result",
	},
	{
		Name:   "F$LDAXY",
		Desc:   "Load A [X+Y]",
		Number: 0x47,
		X:      "base_addr",
		Y:      "offset",
		RA:     "result",
	},
	{
		Name:   "F$LDDDXY",
		Desc:   "Load D [D+X,[Y]]",
		Number: 0x48,
		D:      "offset",
		X:      "base_addr",
		Y:      "dat_image_addr",
		RD:     "result",
	},
	{
		Name:   "F$LDABY",
		Desc:   "Load A from 0,X in task B",
		Number: 0x49,
		B:      "task",
		X:      "addr",
		Y:      "dat_image_addr",
		RA:     "result",
	},
	{
		Name:   "F$STABY",
		Desc:   "Store A into 0,X in task B",
		Number: 0x4A,
		A:      "data",
		B:      "task",
		X:      "addr",
		Y:      "dat_image_addr",
	},
	{
		Name:   "F$LDAXYP",
		Desc:   "Load A from task B at [X+Y]",
		Number: 0x4B,
		B:      "task",
		X:      "base_addr",
		Y:      "offset",
		RA:     "result",
	},
	{
		Name:   "F$STAXYP",
		Desc:   "Store A into task B at [X+Y]",
		Number: 0x4C,
		A:      "data",
		B:      "task",
		X:      "base_addr",
		Y:      "offset",
	},
	{
		Name:   "F$MapBlk",
		Desc:   "Map memory blocks into process space",
		Number: 0x4F,
		B:      "num_blocks",
		X:      "first_block",
		RU:     "addr_of_first_block",
	},
	{
		Name:   "F$ClrRng",
		Desc:   "Clear memory block range",
		Number: 0x50,
		B:      "num_blocks",
		X:      "first_block",
	},

	// I/O Service Request Calls ($80..$91)
	{
		Name:   "I$Attach",
		Desc:   "Attach an I/O device",
		Number: 0x80,
		A:      "access_mode",
		X:      "device_name",
		RX:     "after_name",
		RU:     "device_table_entry_addr",
	},
	{
		Name:   "I$Detach",
		Desc:   "Detach an I/O device",
		Number: 0x81,
		U:      "device_table_entry_addr",
	},
	{
		Name:   "I$Dup",
		Desc:   "Duplicate open path",
		Number: 0x82,
		A:      "path",
		RA:     "new_path",
	},
	{
		Name:   "I$Create",
		Desc:   "Create and open new file",
		Number: 0x83,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Open",
		Desc:   "Open existing file or device",
		Number: 0x84,
		A:      "access_mode",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$MakDir",
		Desc:   "Create a new directory",
		Number: 0x85,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	},
	{
		Name:   "I$ChgDir",
		Desc:   "Change working directory",
		Number: 0x86,
		A:      "access_mode",
		X:      "pathname",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Delete",
		Desc:   "Delete a file or directory",
		Number: 0x87,
		A:      "access_mode",
		X:      "pathname",
		RX:     "after_pathname",
	},
	{
		Name:   "I$Seek",
		Desc:   "Change current file position pointer",
		Number: 0x88,
		A:      "path",
		X:      "upper_16_bits",
		U:      "lower_16_bits",
	},
	{
		Name:   "I$Read",
		Desc:   "Read raw data bytes",
		Number: 0x89,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$Write",
		Desc:   "Write raw data bytes",
		Number: 0x8A,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$ReadLn",
		Desc:   "Read line of ASCII text",
		Number: 0x8B,
		A:      "path",
		X:      "buffer",
		Y:      "max_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$WritLn",
		Desc:   "Write line of ASCII text",
		Number: 0x8C,
		A:      "path",
		X:      "buffer",
		Y:      "num_bytes",
		RY:     "actual_num_bytes",
	},
	{
		Name:   "I$GetStt",
		Desc:   "Get device/file status",
		Number: 0x8D,
		A:      "path",
		B:      "func_code",
		X:      "x_arg",
		Y:      "y_arg",
		U:      "u_arg",
		RD:     "d_result",
		RX:     "x_result",
		RY:     "y_result",
		RU:     "u_result",
	},
	{
		Name:   "I$SetStt",
		Desc:   "Set device/file status",
		Number: 0x8E,
		A:      "path",
		B:      "func_code",
		X:      "x_arg",
		Y:      "y_arg",
		U:      "u_arg",
		RD:     "d_result",
		RX:     "x_result",
		RY:     "y_result",
		RU:     "u_result",
	},
	{
		Name:   "I$Close",
		Desc:   "Close an I/O path",
		Number: 0x8F,
		A:      "path",
	},
	{
		Name:   "I$DeletX",
		Desc:   "Delete file from execution directory",
		Number: 0x90,
		A:      "access_mode",
		X:      "pathname",
		RX:     "after_pathname",
	},
	{
		Name:   "I$ModDsc",
		Desc:   "Modify bytes in file descriptor",
		Number: 0x91,
		B:      "num_bytes",
		X:      "module_name",
		U:      "offset_data_pairs",
	},
}

var ByNumber = make(map[byte]*Call)

var StatusNames = map[byte]string{
	0x00: "SS.Opt (Read/Write path options)",
	0x01: "SS.Ready (Check device ready)",
	0x02: "SS.Size (File size)",
	0x03: "SS.Reset (Device restore)",
	0x05: "SS.Pos (File position)",
	0x06: "SS.EOF (Test EOF)",
	0x07: "SS.Link (Link status routines)",
	0x08: "SS.ULink (Unlink status routines)",
	0x0A: "SS.Frz (Freeze DD info)",
	0x0B: "SS.SPT (Set tracks/cyl)",
	0x0D: "SS.DCmd (Direct disk command)",
	0x0E: "SS.DevNm (Return device name)",
	0x0F: "SS.FD (Return file descriptor)",
	0x10: "SS.Ticks (Set lockout honor duration)",
	0x11: "SS.Lock (Lock/release record)",
	0x14: "SS.BlkRd (Block read)",
	0x15: "SS.BlkWr (Block write)",
	0x16: "SS.Reten (Retension cycle)",
	0x17: "SS.WFM (Write file mark)",
	0x18: "SS.RFM (Read past file mark)",
	0x19: "SS.ELog (Read error log)",
	0x1A: "SS.SSig (Send signal on data ready)",
	0x1B: "SS.Relea (Release device)",
	0x1E: "SS.RsBit (Reserve bitmap sector)",
	0x20: "SS.FDInf (Get FD sector info)",
	0x26: "SS.ScSiz (Get screen size)",
	0x28: "SS.ComSt (Baud/parity settings)",
	0x29: "SS.Open (Device open notify)",
	0x2A: "SS.Close (Device close notify)",
	0x2B: "SS.HngUp (Hang up phone/modem)",
	0x2C: "SS.FSig (Signal for temp locked file)",
	0xA0: "SS.Fill (Enable command-line history)",
}

var SignalNames = map[byte]string{
	0: "S$Kill (Non-interceptable abort)",
	1: "S$Wake (Wake sleeping process)",
	2: "S$Abort (Keyboard abort / ^C)",
	3: "S$Intrpt (Keyboard interrupt)",
}

var ErrorNames = map[byte]string{
	200: "E$PthFul (Path table full)",
	201: "E$BPNum (Bad path number)",
	202: "E$Poll (Polling table full)",
	203: "E$BMode (Bad mode)",
	204: "E$DevOvf (Device table overflow)",
	205: "E$BMID (Bad module ID)",
	206: "E$DirFul (Module directory full)",
	207: "E$MemFul (Process memory full)",
	208: "E$UnkSvc / E$IllFnc (Unknown service code / Illegal function)",
	209: "E$ModBsy (Module busy)",
	210: "E$BPAddr (Bad page address)",
	211: "E$EOF (End of file)",
	212: "E$VctFul (Vector table full)",
	213: "E$NES / E$Format (Non-existent segment / Bad module format)",
	214: "E$FNA / E$BMode (File not accessible / Bad mode)",
	215: "E$BPNam / E$Proc (Bad path name / Process table full)",
	216: "E$PNNF (Path name not found)",
	217: "E$BPNum / E$SLF (Bad path number / Segment list full)",
	218: "E$PTHFUL / E$CEF (Path table full / Creating existing file)",
	219: "E$IBA (Illegal block address)",
	220: "E$HangUp / E$NoPath (Carrier detect lost / No free path)",
	221: "E$MNF (Module not found)",
	222: "E$NoChld (No children)",
	223: "E$DelSP / E$NotRdy (Deleting SP memory / Child not ready)",
	224: "E$IPrcID / E$ProcUndef (Illegal process ID / Process undefined)",
	225: "E$PrcFul (Process table full)",
	226: "E$NoChld (No children)",
	227: "E$ISWI (Illegal SWI code)",
	228: "E$PrcAbt (Process aborted)",
	229: "E$PrcFul (Process table full)",
	230: "E$IForkP (Illegal fork parameter)",
	231: "E$KwnMod (Known module)",
	232: "E$BMCRC (Bad module CRC)",
	233: "E$USigP (Unprocessed signal pending)",
	234: "E$NEMod (Non-existent module)",
	235: "E$BNam (Bad name)",
	236: "E$BMHP (Bad module header parity)",
	237: "E$NoRAM (No system RAM available)",
	238: "E$DNE (Directory not empty)",
	239: "E$NoTask (No available task number)",
	240: "E$Unit (Illegal media unit)",
	241: "E$Sect (Bad sector number)",
	242: "E$WP (Write protect error)",
	243: "E$CRC (Bad checksum/CRC)",
	244: "E$Read (Read error)",
	245: "E$Write (Write error)",
	246: "E$NotRdy (Device not ready)",
	247: "E$Seek (Seek error)",
	248: "E$Full (Media full)",
	249: "E$BTyp (Bad/incompatible media type)",
	250: "E$DevBsy (Device busy)",
	251: "E$DIDC (Media ID change)",
	252: "E$Lock (Record is busy / locked)",
	253: "E$Share (Non-sharable file busy)",
	254: "E$DeadLk (I/O deadlock error)",
}

func init() {
	for _, c := range Calls {
		if ByNumber[c.Number] == nil {
			ByNumber[c.Number] = c
		}
	}
	// Hatvan OS / Level 1 conventions:
	ByNumber[0x85] = &Call{
		Name:   "I$Create",
		Desc:   "Create and open new file",
		Number: 0x85,
		A:      "access_mode",
		B:      "attrs",
		X:      "pathname",
		RA:     "path",
		RX:     "after_pathname",
	}
}

// FindCall returns the Call descriptor for the given OS-9 syscall number.
func FindCall(num byte) *Call {
	return ByNumber[num]
}

// FormatCall returns a pretty-printed string for a syscall invocation.
func FormatCall(c *Call, num byte, a, b byte, d, x, y, u uint16, readStr func(addr uint16) string, readBuf func(addr uint16, maxLen int) string) string {
	if c == nil {
		return fmt.Sprintf("OS9($%02X)", num)
	}

	var args []string
	if c.A != "" {
		if c.A == "path" {
			args = append(args, fmt.Sprintf("path=%d", a))
		} else if strings.Contains(c.A, "mode") {
			args = append(args, fmt.Sprintf("%s=$%02X", c.A, a))
		} else {
			args = append(args, fmt.Sprintf("%s=%d", c.A, a))
		}
	}
	if c.B != "" {
		if (num == 0x8D || num == 0x8E) {
			if stName, ok := StatusNames[b]; ok {
				args = append(args, fmt.Sprintf("func_code=$%02X (%s)", b, stName))
			} else {
				args = append(args, fmt.Sprintf("func_code=$%02X", b))
			}
		} else if num == 0x08 {
			if sigName, ok := SignalNames[b]; ok {
				args = append(args, fmt.Sprintf("signal=$%02X (%s)", b, sigName))
			} else {
				args = append(args, fmt.Sprintf("signal=%d", b))
			}
		} else if c.B == "status" || c.B == "error_code" {
			args = append(args, fmt.Sprintf("%s=%d", c.B, b))
		} else if strings.Contains(c.B, "code") || strings.Contains(c.B, "attr") {
			args = append(args, fmt.Sprintf("%s=$%02X", c.B, b))
		} else {
			args = append(args, fmt.Sprintf("%s=%d", c.B, b))
		}
	}
	if c.D != "" {
		args = append(args, fmt.Sprintf("%s=%d ($%04X)", c.D, d, d))
	}
	if c.X != "" {
		if strings.Contains(c.X, "name") && readStr != nil {
			str := readStr(x)
			args = append(args, fmt.Sprintf("%s=$%04X %q", c.X, x, str))
		} else if c.X == "buffer" {
			args = append(args, fmt.Sprintf("buf=$%04X", x))
		} else {
			args = append(args, fmt.Sprintf("%s=$%04X", c.X, x))
		}
	}
	if c.Y != "" {
		if strings.Contains(c.Y, "bytes") || strings.Contains(c.Y, "size") || strings.Contains(c.Y, "count") {
			args = append(args, fmt.Sprintf("%s=%d", c.Y, y))
		} else {
			args = append(args, fmt.Sprintf("%s=$%04X", c.Y, y))
		}
	}
	if c.U != "" {
		args = append(args, fmt.Sprintf("%s=$%04X", c.U, u))
	}

	res := fmt.Sprintf("%s(%s)", c.Name, strings.Join(args, ", "))

	if (num == 0x8A || num == 0x8C) && readBuf != nil && y > 0 {
		preview := readBuf(x, int(y))
		if preview != "" {
			res += fmt.Sprintf(": %q", preview)
		}
	}

	return res
}

// FormatResult returns a pretty-printed string for a syscall return (after RTI).
func FormatResult(c *Call, num byte, cc, a, b byte, d, x, y, u uint16, bufAddr uint16, readBuf func(addr uint16, maxLen int) string) string {
	name := fmt.Sprintf("OS9($%02X)", num)
	if c != nil {
		name = c.Name
	}

	isError := (cc & 0x01) != 0 // Carry set
	if isError {
		errCode := b
		if errName, ok := ErrorNames[errCode]; ok {
			return fmt.Sprintf("%s: ERROR %d (%s)", name, errCode, errName)
		}
		return fmt.Sprintf("%s: ERROR %d", name, errCode)
	}

	if c == nil {
		return fmt.Sprintf("%s: (OK)", name)
	}

	var resParts []string
	if c.RA != "" {
		if c.RA == "path" || c.RA == "new_path" || c.RA == "child_process_id" || c.RA == "proc_id" {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RA, a))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=%d ($%02X)", c.RA, a, a))
		}
	}
	if c.RB != "" {
		if c.RB == "child_exit_status" {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RB, b))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=%d ($%02X)", c.RB, b, b))
		}
	}
	if c.RD != "" {
		resParts = append(resParts, fmt.Sprintf("%s=%d ($%04X)", c.RD, d, d))
	}
	if c.RX != "" {
		resParts = append(resParts, fmt.Sprintf("%s=$%04X", c.RX, x))
	}
	if c.RY != "" {
		if strings.Contains(c.RY, "bytes") || strings.Contains(c.RY, "size") {
			resParts = append(resParts, fmt.Sprintf("%s=%d", c.RY, y))
		} else {
			resParts = append(resParts, fmt.Sprintf("%s=$%04X", c.RY, y))
		}
	}
	if c.RU != "" {
		resParts = append(resParts, fmt.Sprintf("%s=$%04X", c.RU, u))
	}

	res := ""
	if len(resParts) > 0 {
		res = fmt.Sprintf("%s: %s (OK)", name, strings.Join(resParts, ", "))
	} else {
		res = fmt.Sprintf("%s: (OK)", name)
	}

	if (num == 0x89 || num == 0x8B) && readBuf != nil && y > 0 {
		targetAddr := bufAddr
		if targetAddr == 0 {
			targetAddr = x
		}
		preview := readBuf(targetAddr, int(y))
		if preview != "" {
			res += fmt.Sprintf(": %q", preview)
		}
	}

	return res
}
