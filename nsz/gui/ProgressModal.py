# -*- coding: utf-8 -*-
from kivy.uix.modalview import ModalView
from kivy.properties import StringProperty, NumericProperty
from kivy.clock import Clock
from kivy.lang import Builder
from kivy.event import EventDispatcher

Builder.load_file("nsz/gui/layout/ProgressModal.kv")


class ProgressModal(ModalView):
    operation_name = StringProperty("")
    status_text = StringProperty("Preparing...")
    progress_percent = NumericProperty(0)
    progress_color = [0.2, 0.5, 0.9, 1.0]
    current_file = StringProperty("")
    current_file_index = NumericProperty(0)
    total_files = NumericProperty(0)

    __events__ = ("on_cancel",)

    def __init__(self, **kwargs):
        super(ProgressModal, self).__init__(**kwargs)
        self.auto_dismiss = False
        self.size_hint = (0.7, 0.5)

    def on_cancel(self, *args):
        pass

    def update_status(self, status, progress=None):
        self.status_text = status
        if progress is not None:
            self.progress_percent = progress

    def update_file(self, filename):
        self.current_file = filename

    def set_operation(self, name):
        self.operation_name = name
        self.status_text = "Starting..."
        self.progress_percent = 0
        self.current_file = ""
        self.current_file_index = 0
        self.total_files = 0

    def set_progress_color(self, r, g, b, a=1.0):
        self.progress_color = [r, g, b, a]

    def complete(self, success=True, message="Done!"):
        self.status_text = message
        if success:
            self.progress_percent = 100
            self.set_progress_color(0.2, 0.8, 0.3, 1.0)
        else:
            self.set_progress_color(0.9, 0.3, 0.3, 1.0)

    def close(self, *args):
        self.dismiss()
