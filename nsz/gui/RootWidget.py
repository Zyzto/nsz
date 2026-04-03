import os
import threading
import sys
from os import scandir
from pathlib import Path
from kivy.lang import Builder
from kivy.uix.floatlayout import FloatLayout
from kivy.properties import ObjectProperty, BooleanProperty
from kivy.clock import Clock
from kivy.app import App
from nsz.gui.FileDialogs import *
from nsz.gui.AboutDialog import *
from nsz.gui.ProgressModal import *
from nsz.gui.GuiPath import *
from nsz.PathTools import *
from nsz.nut import Print
from nsz import FileExistingChecks
from nsz.ParseArguments import ParseArguments


class RootWidget(FloatLayout):
    loadfile = ObjectProperty(None)
    savefile = ObjectProperty(None)
    text_input = ObjectProperty(None)
    gameList = None

    is_working = BooleanProperty(False)
    hardExit = True

    C = False
    D = False
    output = False
    verify = False
    info = False
    titlekeys = False
    extract = False
    create = False

    def __init__(self, gameList, **kwargs):
        Builder.load_file(getGuiPath("layout/RootWidget.kv"))
        super(RootWidget, self).__init__(**kwargs)
        self.ids.inFilesLayout.add_widget(gameList)
        self.gameList = gameList
        self.progress_modal = None
        self._cancel_requested = False
        self._operation_thread = None
        self._operation_result = None

    def _create_progress_modal(self):
        if self.progress_modal is None:
            self.progress_modal = ProgressModal()
        return self.progress_modal

    def _run_operation(self, operation_func, operation_name, *args):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return

        self.is_working = True
        modal = self._create_progress_modal()
        modal.set_operation(operation_name)
        modal.set_progress_color(0.2, 0.5, 0.9, 1.0)

        def on_cancel_callback(*cb_args):
            self._cancel_requested = True
            Clock.schedule_once(self._operation_finished)

        modal.bind(on_cancel=on_cancel_callback)
        modal.open()

        self._cancel_requested = False
        self._operation_result = None

        def run_thread():
            try:
                operation_func(*args)
                self._operation_result = (
                    "success",
                    "Operation completed successfully!",
                )
            except Exception as e:
                import traceback

                error_msg = f"Error: {str(e)}"
                Print.error(999, error_msg)
                Print.exception()
                self._operation_result = ("error", error_msg)
            finally:
                Clock.schedule_once(self._operation_finished)

        self._operation_thread = threading.Thread(target=run_thread, daemon=True)
        self._operation_thread.start()

    def _operation_finished(self, dt):
        modal = self._create_progress_modal()

        if self._operation_result:
            status, message = self._operation_result
            if status == "success":
                modal.complete(True, message)
            elif status == "cancelled":
                modal.complete(False, "Operation cancelled")
            else:
                modal.complete(False, message)

        self.is_working = False
        modal.ids.close_btn.opacity = 1
        modal.ids.close_btn.disabled = False
        modal.ids.cancel_btn.opacity = 0
        modal.ids.cancel_btn.disabled = True

        self._reset_flags()

    def _reset_flags(self):
        self.C = False
        self.D = False
        self.verify = False
        self.info = False
        self.titlekeys = False
        self.extract = False
        self.create = False

    def _build_args_and_run(self, args_list, operation_name):
        import subprocess
        import sys
        import os
        import re
        import time

        base_dir = os.path.dirname(os.path.dirname(os.path.dirname(__file__)))
        nsz_path = os.path.join(base_dir, "nsz.py")
        cmd = [sys.executable, nsz_path] + args_list

        process = subprocess.Popen(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1
        )

        modal = self._create_progress_modal()
        current_file = ""

        def update_modal(status=None, progress=None, filename=None):
            def do_update(dt):
                if filename is not None:
                    modal.update_file(filename)
                if status is not None and progress is not None:
                    modal.update_status(status, progress)
                elif status is not None:
                    modal.update_status(status, None)

            Clock.schedule_once(do_update)

        while True:
            line = process.stdout.readline()
            if not line and process.poll() is not None:
                break

            if line:
                print(line.rstrip())

                if "%" in line:
                    percent_match = re.search(r"(\d+)%", line)
                    if percent_match:
                        percent = int(percent_match.group(1))
                        update_modal(status=f"Progress: {percent}%", progress=percent)

                file_match = re.search(
                    r"(?:Compressing|Decompressing|Processing|Verifying)[:\s]+[^\s]+",
                    line,
                )
                if file_match:
                    current_file = file_match.group(0)[:60]
                    update_modal(filename=current_file)

            if self._cancel_requested:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
                self._operation_result = ("cancelled", "Operation cancelled by user")
                return

        return_code = process.wait()

        if return_code == 0:
            self._operation_result = ("success", "Operation completed successfully!")
        else:
            self._operation_result = (
                "error",
                f"Process exited with code {return_code}",
            )

        modal = self._create_progress_modal()
        current_file = ""

        for line in process.stdout:
            print(line.rstrip())

            line_lower = line.lower()

            if "compressing" in line_lower or "decompress" in line_lower:
                percent_match = re.search(r"(\d+)%", line)
                if percent_match:
                    percent = int(percent_match.group(1))
                    Clock.schedule_once(
                        lambda dt, p=percent: modal.update_status(f"Progress: {p}%", p)
                    )

            file_match = re.search(
                r"(?:Compressing|Decompressing|Processing|Verifying)[:\s]+[^\s]+", line
            )
            if file_match:
                current_file = file_match.group(0)[:50]
                Clock.schedule_once(
                    lambda dt, f=current_file: modal.update_status(f, None)
                )

        process.wait()

        if process.returncode == 0:
            self._operation_result = ("success", "Operation completed successfully!")
        else:
            self._operation_result = (
                "error",
                f"Process exited with code {process.returncode}",
            )

    def Compress(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message(
                "No files selected", "Please add files to compress first."
            )
            return
        self.C = True
        args = self._prepare_args()
        self._run_operation(
            self._build_args_and_run, "Compress NSP/XCI", args, "Compress"
        )

    def Decompress(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message(
                "No files selected", "Please add files to decompress first."
            )
            return
        self.D = True
        args = self._prepare_args()
        self._run_operation(
            self._build_args_and_run, "Decompress NSZ/XCZ", args, "Decompress"
        )

    def Verify(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message("No files selected", "Please add files to verify first.")
            return
        self.verify = True
        args = self._prepare_args()
        self._run_operation(self._build_args_and_run, "Verify Files", args, "Verify")

    def Info(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message(
                "No files selected", "Please add files to get info first."
            )
            return
        self.info = True
        args = self._prepare_args()
        self._run_operation(self._build_args_and_run, "Show Info", args, "Info")

    def Titlekeys(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message(
                "No files selected", "Please add files to extract titlekeys first."
            )
            return
        self.titlekeys = True
        args = self._prepare_args()
        self._run_operation(
            self._build_args_and_run, "Extract Titlekeys", args, "Titlekeys"
        )

    def Extract(self):
        if self.is_working:
            Print.warning("An operation is already in progress")
            return
        if not self.gameList.filelist:
            self._show_message(
                "No files selected", "Please add files to extract first."
            )
            return
        self.extract = True
        args = self._prepare_args()
        self._run_operation(self._build_args_and_run, "Extract Files", args, "Extract")

    def _prepare_args(self):
        args = []
        if self.C:
            args.append("-C")
        if self.D:
            args.append("-D")
        if self.verify:
            args.append("--verify")
        if self.info:
            args.append("-i")
        if self.titlekeys:
            args.append("--titlekeys")
        if self.extract:
            args.append("-x")
        if self.create:
            args.append("--create")
        if self.output:
            args.extend(["--output", str(self.output)])

        for filepath in self.gameList.filelist.keys():
            args.append(str(filepath))

        return args

    def _show_message(self, title, message):
        from kivy.uix.label import Label
        from kivy.uix.popup import Popup
        from kivy.uix.boxlayout import BoxLayout
        from kivy.uix.button import Button

        content = BoxLayout(orientation="vertical", padding=20, spacing=10)
        lbl = Label(text=message, font_size=16, text_size=(400, None), halign="center")
        lbl.bind(width=lambda w, h: w.setter("text_size")(w, (w.width, None)))
        content.add_widget(lbl)
        btn = Button(text="OK", size_hint_y=None, height=40)
        content.add_widget(btn)
        popup = Popup(title=title, content=content, size_hint=(0.5, 0.3))
        btn.bind(on_release=popup.dismiss)
        popup.open()

    def dismissPopup(self):
        self._popup.dismiss()

    def showInputFileFolderDialog(self):
        if self.is_working:
            Print.warning("Please wait for the current operation to complete")
            return
        filter = ["*.nsp", "*.nsz", "*.xci", "*.xcz", "*.ncz"]
        content = OpenFileDialog(
            load=self.setInputFileFolder, cancel=self.dismissPopup, filters=filter
        )
        self._popup = Popup(
            title="Input File/Folder", content=content, size_hint=(0.9, 0.9)
        )
        self._popup.open()

    def showOutputFileFolderDialog(self):
        if self.is_working:
            Print.warning("Please wait for the current operation to complete")
            return
        content = OpenFileDialog(
            load=self.setOutputFileFolder,
            cancel=self.dismissPopup,
            filters=[self.showNoFiles],
        )
        self._popup = Popup(
            title="Output File/Folder", content=content, size_hint=(0.9, 0.9)
        )
        self._popup.open()

    def showNoFiles(self, foldername, filename):
        return False

    def setInputFileFolder(self, rawPath, filename):
        if len(filename) == 0:
            return
        path = Path(rawPath).joinpath(filename[0])
        if len(filename) == 1:
            self.gameList.addFiles(path)
        else:
            for file in filename[1:]:
                filepath = path.joinpath(file)
                self.gameList.addFiles(filepath)
        self.dismissPopup()

    def setOutputFileFolder(self, path, filename):
        self.output = path
        Print.info("Set --output to {0}".format(self.output))
        self.dismissPopup()

    def showAboutDialog(self):
        content = AboutDialog(cancel=self.dismissPopup)
        self._popup = Popup(
            title="About", content=content, auto_dismiss=False, size_hint=(0.9, 0.9)
        )
        self._popup.open()
