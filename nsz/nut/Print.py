import sys
import time
import json
from sys import argv
from nsz.ParseArguments import *
from traceback import print_exc

enableInfo = True
enableError = True
enableWarning = True
enableDebug = False
silent = False
verbose = False
machineReadableOutput = False
lastProgress = ""

if len(argv) > 1:
    try:
        args = ParseArguments.parse()
    except:
        args = None

    if args:
        if args.machine_readable:
            machineReadableOutput = True
        if hasattr(args, "verbose") and args.verbose:
            verbose = True
        if hasattr(args, "quiet") and args.quiet:
            enableInfo = False
        if hasattr(args, "silent") and args.silent:
            silent = True


def info(s, pleaseNoPrint=None):
    if silent or not enableInfo:
        return

    if not verbose and not is_verbose_output(s):
        return

    if pleaseNoPrint == None:
        if machineReadableOutput == False:
            sys.stdout.write(s + "\n")
    else:
        if machineReadableOutput == False:
            while pleaseNoPrint.value() > 0:
                time.sleep(0.01)
            pleaseNoPrint.increment()
            sys.stdout.write(s + "\n")
            sys.stdout.flush()
            pleaseNoPrint.decrement()


def is_verbose_output(s):
    if not verbose:
        if s.startswith("[OPEN  ]"):
            return False
        if s.startswith("[NCA "):
            return False
        if s.startswith("[EXISTS]"):
            return False
        if s.startswith("[NCZBLOCK]"):
            return False
    return True


def error(errorCode, s):
    if silent or not enableError:
        return
    if machineReadableOutput:
        s = json.dumps({"error": s, "errorCode": errorCode, "warning": False})

    sys.stdout.write(s + "\n")


def warning(s):
    if silent or not enableWarning:
        return
    if machineReadableOutput:
        s = json.dumps({"error": False, "warning": s})

    sys.stdout.write(s + "\n")


def debug(s):
    if silent or not enableDebug:
        return
    if machineReadableOutput == False:
        sys.stdout.write(s + "\n")


def exception():
    if machineReadableOutput == False:
        print_exc()


def progress(job, s):
    global lastProgress

    if machineReadableOutput:
        s = json.dumps({"job": job, "data": s, "error": False, "warning": False})

        if s != lastProgress:
            sys.stdout.write(s + "\n")

            lastProgress = s
