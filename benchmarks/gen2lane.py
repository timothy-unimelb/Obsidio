#!/usr/bin/env python3
"""Generate app/sha256block2_amd64.s: two-lane interleaved SHA-NI single-block
compression, mechanically derived from the vendored stdlib blockSHANI routine.

Lane A registers: state X1,X2; W X3,X4,X5,X6; temp X7; data SI; dig DI
Lane B registers: state X8,X9; W X10,X11,X12,X13; temp X15; data R9; dig R10
Shared: X0 (implicit SHA256RNDS2 WK operand, reloaded per group), X14 (flip
mask), AX (K256 table), stack 0..63 (saved initial states for feedforward).

Interleaving is at 4-round-group granularity: each lane's WK lives in X0 only
within its own group, so the lanes never fight over the implicit register and
the OOO core overlaps the two dependency chains.
"""

# Single-lane 4-round groups transcribed from the vendored blockSHANI
# (app/sha256block_amd64.s lines 1315-1465). Register placeholders:
# S1,S2 = state; W0,W1,W2,W3 = current rotation of msg regs; T = temp;
# F = flip mask; DATA = message pointer.
LOAD_GROUPS = [
    # (data offset, K offset, W target, trailing msg1 (dst,src) or None, mid schedule or None)
    (0,   0,   "W0", None,          None),
    (16,  32,  "W1", ("W0", "W1"),  None),
    (32,  64,  "W2", ("W1", "W2"),  None),
    (48,  96,  "W3", ("W2", "W3"),  ("W3", "W2", "W0")),  # mid: T=W3;palignr W2;paddd->W0? see emit
]
# Middle groups: (K offset, src W, sched dst W, palignr src W, msg1 pair or None)
MID_GROUPS = [
    (128, "W0", "W1", "W3", ("W3", "W0")),
    (160, "W1", "W2", "W0", ("W0", "W1")),
    (192, "W2", "W3", "W1", ("W1", "W2")),
    (224, "W3", "W0", "W2", ("W2", "W3")),
    (256, "W0", "W1", "W3", ("W3", "W0")),
    (288, "W1", "W2", "W0", ("W0", "W1")),
    (320, "W2", "W3", "W1", ("W1", "W2")),
    (352, "W3", "W0", "W2", ("W2", "W3")),
    (384, "W0", "W1", "W3", ("W3", "W0")),
    (416, "W1", "W2", "W0", None),
    (448, "W2", "W3", "W1", None),
]
FINAL_K = 480  # rounds 60-63, src W3


def lane_regs(lane):
    if lane == "A":
        return {"S1": "X1", "S2": "X2", "W0": "X3", "W1": "X4", "W2": "X5",
                "W3": "X6", "T": "X7", "DATA": "SI", "DIG": "DI",
                "SAVE1": "(SP)", "SAVE2": "16(SP)"}
    return {"S1": "X8", "S2": "X9", "W0": "X10", "W1": "X11", "W2": "X12",
            "W3": "X13", "T": "X15", "DATA": "R9", "DIG": "R10",
            "SAVE1": "32(SP)", "SAVE2": "48(SP)"}


def rnds2_pair(r, out):
    out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
    out.append("\tPSHUFD      $0x0e, X0, X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")


def emit_load_group(lane, idx, out):
    r = lane_regs(lane)
    off, koff, wt, msg1, mid = LOAD_GROUPS[idx]
    w = r[wt]
    src = f"({r['DATA']})" if off == 0 else f"{off}({r['DATA']})"
    out.append(f"\t// lane {lane} rounds {idx*4}-{idx*4+3}")
    out.append(f"\tVMOVDQU     {src}, X0")
    out.append("\tPSHUFB      X14, X0")
    out.append(f"\tVMOVDQA     X0, {w}")
    kop = "(AX)" if koff == 0 else f"{koff}(AX)"
    out.append(f"\tPADDD       {kop}, X0")
    if mid is None:
        rnds2_pair(r, out)
    else:
        # group 3 pattern: first rnds2, schedule W0 via palignr/msg2, then second
        out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
        out.append(f"\tVMOVDQA     {r['W3']}, {r['T']}")
        out.append(f"\tPALIGNR     $0x04, {r['W2']}, {r['T']}")
        out.append(f"\tPADDD       {r['T']}, {r['W0']}")
        out.append(f"\tSHA256MSG2  {r['W3']}, {r['W0']}")
        out.append("\tPSHUFD      $0x0e, X0, X0")
        out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")
    if msg1 is not None:
        out.append(f"\tSHA256MSG1  {r[msg1[1]]}, {r[msg1[0]]}")


def emit_mid_group(lane, gi, out):
    r = lane_regs(lane)
    koff, wsrc, wdst, walign, msg1 = MID_GROUPS[gi]
    rounds = 16 + gi * 4
    out.append(f"\t// lane {lane} rounds {rounds}-{rounds+3}")
    out.append(f"\tVMOVDQA     {r[wsrc]}, X0")
    out.append(f"\tPADDD       {koff}(AX), X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S1']}, {r['S2']}")
    out.append(f"\tVMOVDQA     {r[wsrc]}, {r['T']}")
    out.append(f"\tPALIGNR     $0x04, {r[walign]}, {r['T']}")
    out.append(f"\tPADDD       {r['T']}, {r[wdst]}")
    out.append(f"\tSHA256MSG2  {r[wsrc]}, {r[wdst]}")
    out.append("\tPSHUFD      $0x0e, X0, X0")
    out.append(f"\tSHA256RNDS2 X0, {r['S2']}, {r['S1']}")
    if msg1 is not None:
        out.append(f"\tSHA256MSG1  {r[msg1[1]]}, {r[msg1[0]]}")


def emit_final_group(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// lane {lane} rounds 60-63")
    out.append(f"\tVMOVDQA     {r['W3']}, X0")
    out.append(f"\tPADDD       {FINAL_K}(AX), X0")
    rnds2_pair(r, out)


def emit_state_load(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// load lane {lane} state, shuffle to ABEF/CDGH, save for feedforward")
    out.append(f"\tVMOVDQU ({r['DIG']}), {r['S1']}")
    out.append(f"\tVMOVDQU 16({r['DIG']}), {r['S2']}")
    out.append(f"\tPSHUFD  $0xb1, {r['S1']}, {r['S1']}")
    out.append(f"\tPSHUFD  $0x1b, {r['S2']}, {r['S2']}")
    out.append(f"\tVMOVDQA {r['S1']}, {r['T']}")
    out.append(f"\tPALIGNR $0x08, {r['S2']}, {r['S1']}")
    out.append(f"\tPBLENDW $0xf0, {r['T']}, {r['S2']}")
    out.append(f"\tVMOVDQU {r['S1']}, {r['SAVE1']}")
    out.append(f"\tVMOVDQU {r['S2']}, {r['SAVE2']}")


def emit_state_store(lane, out):
    r = lane_regs(lane)
    out.append(f"\t// lane {lane}: feedforward + unshuffle + store")
    out.append(f"\tVMOVDQU {r['SAVE1']}, {r['T']}")
    out.append(f"\tPADDD   {r['T']}, {r['S1']}")
    out.append(f"\tVMOVDQU {r['SAVE2']}, {r['T']}")
    out.append(f"\tPADDD   {r['T']}, {r['S2']}")
    out.append(f"\tPSHUFD  $0x1b, {r['S1']}, {r['S1']}")
    out.append(f"\tPSHUFD  $0xb1, {r['S2']}, {r['S2']}")
    out.append(f"\tVMOVDQA {r['S1']}, {r['T']}")
    out.append(f"\tPBLENDW $0xf0, {r['S2']}, {r['S1']}")
    out.append(f"\tPALIGNR $0x08, {r['T']}, {r['S2']}")
    out.append(f"\tVMOVDQU {r['S1']}, ({r['DIG']})")
    out.append(f"\tVMOVDQU {r['S2']}, 16({r['DIG']})")


out = []
out.append("// Code generated by scratchpad/gen2lane.py — two-lane interleave of the")
out.append("// vendored stdlib blockSHANI (sha256block_amd64.s). DO NOT EDIT BY HAND.")
out.append("// Requires: AVX, SHA, SSE4.1, SSSE3 (same gate as blockSHANI).")
out.append("")
out.append("//go:build !purego")
out.append("")
out.append('#include "textflag.h"')
out.append("")
out.append("// func blockSHANI2(da *shaDigest, db *shaDigest, pa *[64]byte, pb *[64]byte)")
out.append("TEXT ·blockSHANI2(SB), NOSPLIT, $64-32")
out.append("\tMOVQ    da+0(FP), DI")
out.append("\tMOVQ    db+8(FP), R10")
out.append("\tMOVQ    pa+16(FP), SI")
out.append("\tMOVQ    pb+24(FP), R9")
out.append("\tVMOVDQA flip_mask<>+0(SB), X14")
out.append("\tLEAQ    K256<>+0(SB), AX")
emit_state_load("A", out)
emit_state_load("B", out)
for i in range(len(LOAD_GROUPS)):
    emit_load_group("A", i, out)
    emit_load_group("B", i, out)
for g in range(len(MID_GROUPS)):
    emit_mid_group("A", g, out)
    emit_mid_group("B", g, out)
emit_final_group("A", out)
emit_final_group("B", out)
emit_state_store("A", out)
emit_state_store("B", out)
out.append("\tRET")
out.append("")

# K256<> and flip_mask<> are FILE-LOCAL in Go asm — this file needs its own
# copies, extracted verbatim from the vendored stdlib file.
import re
import sys

with open("app/sha256block_amd64.s") as f:
    vendored = f.read()
for sym in ("flip_mask", "K256"):
    lines = [l for l in vendored.split("\n")
             if re.match(rf"^(DATA|GLOBL) {sym}<>", l)]
    if not lines:
        raise SystemExit(f"symbol {sym} not found in vendored asm")
    out.append(f"// {sym}: verbatim copy from sha256block_amd64.s (file-local there)")
    out.extend(lines)
    out.append("")
path = sys.argv[1] if len(sys.argv) > 1 else "sha256block2_amd64.s"
with open(path, "w") as f:
    f.write("\n".join(out))
print(f"wrote {path}: {len(out)} lines")
