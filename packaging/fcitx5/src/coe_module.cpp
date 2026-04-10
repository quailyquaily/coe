#include <dbus/dbus.h>

#include <algorithm>
#include <atomic>
#include <cctype>
#include <chrono>
#include <cstdlib>
#include <fstream>
#include <memory>
#include <optional>
#include <sstream>
#include <string>
#include <thread>
#include <unistd.h>
#include <vector>

#include <fcitx/addonfactory.h>
#include <fcitx/addoninstance.h>
#include <fcitx/addonmanager.h>
#include <fcitx/event.h>
#include <fcitx/inputcontext.h>
#include <fcitx/inputpanel.h>
#include <fcitx-utils/event.h>
#include <fcitx-utils/eventdispatcher.h>
#include <fcitx/instance.h>
#include <fcitx-utils/key.h>
#include <fcitx-utils/log.h>
#include <fcitx-utils/utf8.h>

namespace {

constexpr const char *kServiceName = "com.mistermorph.Coe";
constexpr const char *kObjectPath = "/com/mistermorph/Coe";
constexpr const char *kInterfaceName = "com.mistermorph.Coe.Dictation1";
constexpr const char *kDefaultTriggerKey = "Shift+Super+D";
constexpr const char *kDefaultTriggerMode = "toggle";
constexpr const char *kEscapeKey = "Escape";

std::string debugMarkerPath() {
    std::ostringstream out;
    out << "/tmp/coe-fcitx-" << getuid() << ".log";
    return out.str();
}

void appendDebugMarker(const std::string &message) {
    std::ofstream out(debugMarkerPath(), std::ios::app);
    if (!out.is_open()) {
        return;
    }
    const auto now = std::chrono::system_clock::now();
    const auto millis =
        std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch())
            .count();
    out << millis << " " << message << "\n";
}

std::string trim(std::string value) {
    while (!value.empty() &&
           std::isspace(static_cast<unsigned char>(value.front()))) {
        value.erase(value.begin());
    }
    while (!value.empty() &&
           std::isspace(static_cast<unsigned char>(value.back()))) {
        value.pop_back();
    }
    return value;
}

std::string toLower(std::string value) {
    for (auto &ch : value) {
        ch = static_cast<char>(std::tolower(static_cast<unsigned char>(ch)));
    }
    return value;
}

std::string normalizeTriggerMode(std::string value) {
    value = toLower(trim(value));
    if (value.empty()) {
        return kDefaultTriggerMode;
    }
    return value;
}

bool isSupportedTriggerMode(const std::string &value) {
    const auto normalized = normalizeTriggerMode(value);
    return normalized == "toggle" || normalized == "hold";
}

struct TriggerKeyConfig {
    fcitx::Key key;
    std::string source;
    std::string warning;
};

struct SelectionEditTarget {
    fcitx::TrackableObjectReference<fcitx::InputContext> inputContext;
    unsigned int anchor = 0;
    unsigned int cursor = 0;
    std::string selectedText;
};

struct ActiveSelectionEditTarget {
    std::string sessionID;
    SelectionEditTarget target;
};

std::string selectionTextFromSurrounding(
    const fcitx::SurroundingText &surrounding) {
    if (!surrounding.isValid()) {
        return "";
    }

    auto selectedText = surrounding.selectedText();
    if (!selectedText.empty()) {
        return selectedText;
    }

    const auto start = std::min(surrounding.anchor(), surrounding.cursor());
    const auto end = std::max(surrounding.anchor(), surrounding.cursor());
    if (start == end) {
        return "";
    }

    const auto &text = surrounding.text();
    if (!fcitx::utf8::validate(text)) {
        return "";
    }
    const auto length = fcitx::utf8::lengthValidated(text);
    if (end > length) {
        return "";
    }

    const auto beginIter = fcitx::utf8::nextNChar(text.begin(), start);
    const auto endIter = fcitx::utf8::nextNChar(text.begin(), end);
    return std::string(beginIter, endIter);
}

std::string normalizeModifierToken(const std::string &token) {
    auto lowered = token;
    for (auto &ch : lowered) {
        ch = static_cast<char>(std::tolower(static_cast<unsigned char>(ch)));
    }
    if (lowered == "ctrl" || lowered == "control") {
        return "Control";
    }
    if (lowered == "alt") {
        return "Alt";
    }
    if (lowered == "shift") {
        return "Shift";
    }
    if (lowered == "super" || lowered == "meta" || lowered == "win" ||
        lowered == "windows") {
        return "Super";
    }
    return token;
}

std::string normalizeKeyToken(const std::string &token) {
    if (token.empty()) {
        return "";
    }
    if (token.size() == 1 &&
        std::isalpha(static_cast<unsigned char>(token.front()))) {
        return std::string(
            1, static_cast<char>(std::toupper(static_cast<unsigned char>(token.front()))));
    }

    auto lowered = token;
    for (auto &ch : lowered) {
        ch = static_cast<char>(std::tolower(static_cast<unsigned char>(ch)));
    }
    if (lowered == "ctrl") {
        return "Control";
    }
    if (lowered == "enter") {
        return "Return";
    }
    if (lowered == "esc") {
        return "Escape";
    }
    if (lowered.size() >= 2 && lowered.front() == 'f' &&
        std::isdigit(static_cast<unsigned char>(lowered[1]))) {
        auto normalized = lowered;
        normalized[0] = 'F';
        return normalized;
    }
    return token;
}

std::string acceleratorToFcitxKeyString(const std::string &value) {
    auto accelerator = trim(value);
    if (accelerator.empty()) {
        return "";
    }
    if (accelerator.front() != '<') {
        return accelerator;
    }

    std::vector<std::string> parts;
    while (!accelerator.empty() && accelerator.front() == '<') {
        const auto end = accelerator.find('>');
        if (end == std::string::npos) {
            return "";
        }
        auto token = trim(accelerator.substr(1, end - 1));
        if (token.empty()) {
            return "";
        }
        parts.push_back(normalizeModifierToken(token));
        accelerator = trim(accelerator.substr(end + 1));
    }

    auto key = normalizeKeyToken(trim(accelerator));
    if (key.empty()) {
        return "";
    }
    parts.push_back(key);

    std::ostringstream out;
    for (size_t i = 0; i < parts.size(); ++i) {
        if (i > 0) {
            out << "+";
        }
        out << parts[i];
    }
    return out.str();
}

std::string panelTextForState(const std::string &state) {
    if (state == "recording") {
        return "●";
    }
    if (state == "processing") {
        return "◐";
    }
    return "";
}

bool shouldSuppressErrorPanel(const std::string &message) {
    const auto lowered = toLower(trim(message));
    if (lowered.empty()) {
        return true;
    }
    const std::vector<std::string> ignoredSubstrings = {
        "near-silent",
        "no speech",
        "returned empty text",
        "produced no text",
        "saturated or corrupted",
    };
    for (const auto &needle : ignoredSubstrings) {
        if (lowered.find(needle) != std::string::npos) {
            return true;
        }
    }
    return false;
}

std::string shortErrorLabel(const std::string &message) {
    const auto lowered = toLower(message);
    const std::vector<std::pair<std::string, std::string>> mappings = {
        {"recording start failed", "× Mic"},
        {"recording stop failed", "× Mic"},
        {"transcription failed", "× ASR"},
        {"asr", "× ASR"},
        {"correction", "× LLM"},
        {"cleanup", "× LLM"},
        {"dbus", "× D-Bus"},
        {"not ready", "× Not ready"},
    };
    for (const auto &[needle, label] : mappings) {
        if (lowered.find(needle) != std::string::npos) {
            return label;
        }
    }
    return "× Failed";
}

class CoeModule final : public fcitx::AddonInstance {
public:
    explicit CoeModule(fcitx::Instance *instance)
        : instance_(instance),
          triggerKey_(fcitx::Key(kDefaultTriggerKey).normalize()),
          triggerKeySource_("default"),
          triggerMode_(kDefaultTriggerMode),
          dictationState_("idle") {
        if (!instance_) {
            FCITX_ERROR() << "coe-fcitx: missing fcitx instance";
            appendDebugMarker("init error missing-instance");
            return;
        }

        if (!triggerKey_.isValid()) {
            FCITX_ERROR() << "coe-fcitx: invalid trigger key";
            appendDebugMarker("init error invalid-trigger-key");
            return;
        }

        dbus_threads_init_default();
        dispatcher_.attach(&instance_->eventLoop());
        connectCallBus();
        refreshTriggerKeyFromDaemon();
        refreshTriggerModeFromDaemon();
        connectSignalBus();
        keyWatcher_ = instance_->watchEvent(
            fcitx::EventType::InputContextKeyEvent,
            fcitx::EventWatcherPhase::PreInputMethod,
            [this](fcitx::Event &event) { this->handleKeyEvent(event); });
        surroundingWatcher_ = instance_->watchEvent(
            fcitx::EventType::InputContextSurroundingTextUpdated,
            fcitx::EventWatcherPhase::Default,
            [this](fcitx::Event &event) {
                this->handleSurroundingTextUpdated(event);
            });
        focusOutWatcher_ = instance_->watchEvent(
            fcitx::EventType::InputContextFocusOut,
            fcitx::EventWatcherPhase::Default,
            [this](fcitx::Event &event) { this->handleFocusOut(event); });
        startSignalLoop();
        FCITX_INFO() << "coe-fcitx: module initialized with trigger "
                     << triggerKey_.toString() << " source="
                     << triggerKeySource_ << " mode=" << triggerMode_;
        appendDebugMarker("init ok trigger=" + triggerKey_.toString() +
                          " source=" + triggerKeySource_ +
                          " mode=" + triggerMode_);
    }

    ~CoeModule() override {
        running_ = false;
        if (signalThread_.joinable()) {
            signalThread_.join();
        }
        closeBus(signalBus_);
        closeBus(callBus_);
        dispatcher_.detach();
    }

private:
    static void closeBus(DBusConnection *&bus) {
        if (!bus) {
            return;
        }
        dbus_connection_close(bus);
        dbus_connection_unref(bus);
        bus = nullptr;
    }

    DBusConnection *connectPrivateBus(const char *purpose) {
        DBusError err;
        dbus_error_init(&err);
        DBusConnection *bus = dbus_bus_get_private(DBUS_BUS_SESSION, &err);
        if (dbus_error_is_set(&err)) {
            FCITX_ERROR() << "coe-fcitx: failed to connect " << purpose
                          << " bus: "
                          << err.message;
            dbus_error_free(&err);
            return nullptr;
        }
        if (!bus) {
            FCITX_ERROR() << "coe-fcitx: session bus is unavailable for "
                          << purpose;
            return nullptr;
        }
        dbus_connection_set_exit_on_disconnect(bus, false);
        return bus;
    }

    void connectCallBus() { callBus_ = connectPrivateBus("call"); }

    void refreshTriggerKeyFromDaemon() {
        if (!callBus_) {
            return;
        }

        DBusMessage *message = dbus_message_new_method_call(
            kServiceName, kObjectPath, kInterfaceName, "TriggerKey");
        if (!message) {
            FCITX_WARN() << "coe-fcitx: failed to allocate TriggerKey() D-Bus message";
            return;
        }

        DBusError err;
        dbus_error_init(&err);
        DBusMessage *reply = dbus_connection_send_with_reply_and_block(
            callBus_, message, 1500, &err);
        dbus_message_unref(message);

        if (dbus_error_is_set(&err)) {
            FCITX_WARN() << "coe-fcitx: TriggerKey() failed, using default: "
                         << err.name << " " << err.message;
            appendDebugMarker("trigger-key fallback default dbus-error");
            dbus_error_free(&err);
            return;
        }
        if (!reply) {
            FCITX_WARN() << "coe-fcitx: TriggerKey() returned no reply, using default";
            appendDebugMarker("trigger-key fallback default no-reply");
            return;
        }

        const char *triggerKey = "";
        if (!dbus_message_get_args(reply, &err, DBUS_TYPE_STRING, &triggerKey,
                                   DBUS_TYPE_INVALID)) {
            FCITX_WARN() << "coe-fcitx: failed to parse TriggerKey() reply, using default: "
                         << err.message;
            appendDebugMarker("trigger-key fallback default parse-error");
            dbus_error_free(&err);
            dbus_message_unref(reply);
            return;
        }

        const auto rawValue = std::string(triggerKey ? triggerKey : "");
        const auto fcitxKeyString = acceleratorToFcitxKeyString(rawValue);
        auto parsedKey = fcitx::Key(fcitxKeyString).normalize();
        if (!parsedKey.isValid()) {
            FCITX_WARN() << "coe-fcitx: invalid trigger key from daemon '"
                         << rawValue << "', using default";
            appendDebugMarker("trigger-key fallback default invalid-daemon-value");
            dbus_message_unref(reply);
            return;
        }

        triggerKey_ = parsedKey;
        triggerKeySource_ = "daemon";
        appendDebugMarker("trigger-key loaded raw=" + rawValue + " fcitx=" +
                          triggerKey_.toString());
        dbus_message_unref(reply);
    }

    void refreshTriggerModeFromDaemon() {
        if (!callBus_) {
            return;
        }

        DBusMessage *message = dbus_message_new_method_call(
            kServiceName, kObjectPath, kInterfaceName, "TriggerMode");
        if (!message) {
            FCITX_WARN() << "coe-fcitx: failed to allocate TriggerMode() D-Bus message";
            return;
        }

        DBusError err;
        dbus_error_init(&err);
        DBusMessage *reply = dbus_connection_send_with_reply_and_block(
            callBus_, message, 1500, &err);
        dbus_message_unref(message);

        if (dbus_error_is_set(&err)) {
            FCITX_WARN() << "coe-fcitx: TriggerMode() failed, using default: "
                         << err.name << " " << err.message;
            appendDebugMarker("trigger-mode fallback default dbus-error");
            dbus_error_free(&err);
            return;
        }
        if (!reply) {
            FCITX_WARN() << "coe-fcitx: TriggerMode() returned no reply, using default";
            appendDebugMarker("trigger-mode fallback default no-reply");
            return;
        }

        const char *triggerMode = "";
        if (!dbus_message_get_args(reply, &err, DBUS_TYPE_STRING, &triggerMode,
                                   DBUS_TYPE_INVALID)) {
            FCITX_WARN() << "coe-fcitx: failed to parse TriggerMode() reply, using default: "
                         << err.message;
            appendDebugMarker("trigger-mode fallback default parse-error");
            dbus_error_free(&err);
            dbus_message_unref(reply);
            return;
        }

        const auto rawValue = std::string(triggerMode ? triggerMode : "");
        const auto normalized = normalizeTriggerMode(rawValue);
        if (!isSupportedTriggerMode(normalized)) {
            FCITX_WARN() << "coe-fcitx: invalid trigger mode from daemon '"
                         << rawValue << "', using default";
            appendDebugMarker("trigger-mode fallback default invalid-daemon-value");
            dbus_message_unref(reply);
            return;
        }

        triggerMode_ = normalized;
        appendDebugMarker("trigger-mode loaded value=" + triggerMode_);
        dbus_message_unref(reply);
    }

    void connectSignalBus() {
        signalBus_ = connectPrivateBus("signal");
        if (!signalBus_) {
            return;
        }

        addSignalMatch(
            "type='signal',sender='com.mistermorph.Coe',interface='com.mistermorph.Coe.Dictation1',member='StateChanged',path='/com/mistermorph/Coe'");
        addSignalMatch(
            "type='signal',sender='com.mistermorph.Coe',interface='com.mistermorph.Coe.Dictation1',member='ResultReady',path='/com/mistermorph/Coe'");
        addSignalMatch(
            "type='signal',sender='com.mistermorph.Coe',interface='com.mistermorph.Coe.Dictation1',member='ErrorRaised',path='/com/mistermorph/Coe'");
    }

    void addSignalMatch(const char *rule) {
        if (!signalBus_) {
            return;
        }
        DBusError err;
        dbus_error_init(&err);
        dbus_bus_add_match(signalBus_, rule, &err);
        dbus_connection_flush(signalBus_);
        if (dbus_error_is_set(&err)) {
            FCITX_WARN() << "coe-fcitx: failed to add D-Bus match: "
                         << err.message;
            dbus_error_free(&err);
        }
    }

    void startSignalLoop() {
        if (!signalBus_) {
            return;
        }
        running_ = true;
        signalThread_ = std::thread([this]() { this->signalLoop(); });
    }

    void signalLoop() {
        while (running_) {
            dbus_connection_read_write(signalBus_, 200);
            while (DBusMessage *message = dbus_connection_pop_message(signalBus_)) {
                handleSignal(message);
                dbus_message_unref(message);
            }
        }
    }

    void handleSignal(DBusMessage *message) {
        if (!message) {
            return;
        }

        if (dbus_message_is_signal(message, kInterfaceName, "StateChanged")) {
            const char *state = "";
            const char *sessionID = "";
            const char *detail = "";
            DBusError err;
            dbus_error_init(&err);
            if (!dbus_message_get_args(message, &err, DBUS_TYPE_STRING, &state,
                                       DBUS_TYPE_STRING, &sessionID,
                                       DBUS_TYPE_STRING, &detail,
                                       DBUS_TYPE_INVALID)) {
                FCITX_WARN() << "coe-fcitx: failed to parse StateChanged: "
                             << err.message;
                dbus_error_free(&err);
                return;
            }
            auto stateText = std::string(state ? state : "");
            auto detailText = std::string(detail ? detail : "");
            FCITX_DEBUG() << "coe-fcitx: state changed to " << stateText;
            appendDebugMarker("state " + stateText);
            auto sessionValue = std::string(sessionID ? sessionID : "");
            dispatcher_.schedule([this, stateText, sessionValue, detailText]() {
                if (stateText == "recording") {
                    this->bindPendingSelectionEdit(sessionValue);
                } else if (stateText == "idle") {
                    this->clearSelectionEditTargets();
                }
                this->updatePanelState(stateText, detailText);
            });
            return;
        }

        if (dbus_message_is_signal(message, kInterfaceName, "ResultReady")) {
            const char *sessionID = "";
            const char *text = "";
            DBusError err;
            dbus_error_init(&err);
            if (!dbus_message_get_args(message, &err, DBUS_TYPE_STRING,
                                       &sessionID, DBUS_TYPE_STRING, &text,
                                       DBUS_TYPE_INVALID)) {
                FCITX_WARN() << "coe-fcitx: failed to parse ResultReady: "
                             << err.message;
                dbus_error_free(&err);
                return;
            }
            FCITX_DEBUG() << "coe-fcitx: received ResultReady for session "
                          << (sessionID ? sessionID : "") << " with "
                          << (text ? std::string(text).size() : 0)
                          << " bytes";
            appendDebugMarker(
                std::string("result session=") + (sessionID ? sessionID : "") +
                " bytes=" +
                std::to_string(text ? std::string(text).size() : 0));
            auto committedText = std::string(text ? text : "");
            auto sessionValue = std::string(sessionID ? sessionID : "");
            dispatcher_.schedule([this, sessionValue, committedText]() {
                this->commitResult(sessionValue, committedText);
            });
            return;
        }

        if (dbus_message_is_signal(message, kInterfaceName, "ErrorRaised")) {
            const char *sessionID = "";
            const char *errorText = "";
            DBusError err;
            dbus_error_init(&err);
            if (!dbus_message_get_args(message, &err, DBUS_TYPE_STRING,
                                       &sessionID, DBUS_TYPE_STRING,
                                       &errorText, DBUS_TYPE_INVALID)) {
                FCITX_WARN() << "coe-fcitx: failed to parse ErrorRaised: "
                             << err.message;
                dbus_error_free(&err);
                return;
            }
            FCITX_WARN() << "coe-fcitx: daemon error for session "
                         << (sessionID ? sessionID : "") << ": "
                         << (errorText ? errorText : "");
            auto errorValue = std::string(errorText ? errorText : "");
            appendDebugMarker("error " + errorValue);
            dispatcher_.schedule([this, errorValue]() {
                this->activeSelectionEdit_.reset();
                this->showErrorPanel(errorValue);
            });
        }
    }

    void updatePanelState(const std::string &state, const std::string &) {
        dictationState_ = state;
        if (state != "recording") {
            holding_ = false;
        }
        if (state == "recording") {
            startPanelAnimation("recording");
            return;
        }
        if (state == "processing") {
            startPanelAnimation("processing");
            return;
        }
        stopPanelAnimation();
        clearPanel();
        appendDebugMarker("panel clear state=" + state);
    }

    void startPanelAnimation(const std::string &state) {
        animatedPanelState_ = state;
        panelFrame_ = 0;
        cancelClearPanel();
        renderAnimatedPanelFrame();
        ensureAnimationTimer();
    }

    void stopPanelAnimation() {
        animatedPanelState_.clear();
        panelFrame_ = 0;
        if (animationTimer_) {
            animationTimer_->setEnabled(false);
        }
    }

    void ensureAnimationTimer() {
        const uint64_t intervalUsec = 350000;
        if (!animationTimer_) {
            animationTimer_ = instance_->eventLoop().addTimeEvent(
                CLOCK_MONOTONIC, fcitx::now(CLOCK_MONOTONIC) + intervalUsec,
                50000,
                [this](fcitx::EventSourceTime *event, uint64_t) {
                    if (animatedPanelState_.empty()) {
                        return false;
                    }
                    renderAnimatedPanelFrame();
                    event->setNextInterval(intervalUsec);
                    return true;
                });
        }
        animationTimer_->setEnabled(true);
    }

    void cancelClearPanel() {
        if (clearTimer_) {
            clearTimer_->setEnabled(false);
        }
    }

    void scheduleClearPanel(uint64_t delayUsec) {
        cancelClearPanel();
        if (!clearTimer_) {
            clearTimer_ = instance_->eventLoop().addTimeEvent(
                CLOCK_MONOTONIC, fcitx::now(CLOCK_MONOTONIC) + delayUsec,
                50000,
                [this](fcitx::EventSourceTime *, uint64_t) {
                    clearPanel();
                    return false;
                });
        } else {
            clearTimer_->setTime(fcitx::now(CLOCK_MONOTONIC) + delayUsec);
            clearTimer_->setEnabled(true);
        }
    }

    void renderAnimatedPanelFrame() {
        const auto text = currentAnimatedPanelText();
        if (text.empty()) {
            clearPanel();
            return;
        }
        showPanel(text);
        panelFrame_++;
    }

    std::string currentAnimatedPanelText() const {
        if (animatedPanelState_ == "recording") {
            return (panelFrame_ % 2 == 0) ? "●" : "○";
        }
        if (animatedPanelState_ == "processing") {
            static const std::vector<std::string> frames = {"◐", "◓", "◑", "◒"};
            return frames[panelFrame_ % frames.size()];
        }
        return "";
    }

    void showErrorPanel(const std::string &message) {
        stopPanelAnimation();
        if (shouldSuppressErrorPanel(message)) {
            clearPanel();
            appendDebugMarker("panel suppress error");
            return;
        }
        const auto label = shortErrorLabel(message);
        showPanel(label);
        scheduleClearPanel(1500000);
        appendDebugMarker("panel error text=" + label);
    }

    void showPanel(const std::string &text) {
        auto *inputContext = instance_->lastFocusedInputContext();
        if (!inputContext || !inputContext->hasFocus()) {
            appendDebugMarker("panel skipped no-focused-input-context");
            return;
        }

        fcitx::Text auxText;
        auxText.append(text);
        inputContext->inputPanel().setAuxUp(auxText);
        inputContext->updateUserInterface(
            fcitx::UserInterfaceComponent::InputPanel, true);
        FCITX_DEBUG() << "coe-fcitx: panel text " << text;
    }

    void clearPanel() {
        cancelClearPanel();
        auto *inputContext = instance_->lastFocusedInputContext();
        if (!inputContext || !inputContext->hasFocus()) {
            return;
        }
        fcitx::Text auxText;
        inputContext->inputPanel().setAuxUp(auxText);
        inputContext->updateUserInterface(
            fcitx::UserInterfaceComponent::InputPanel, true);
    }

    std::optional<SelectionEditTarget> captureSelectionEditTarget(
        fcitx::InputContext *inputContext) {
        if (!inputContext || !inputContext->hasFocus()) {
            return std::nullopt;
        }

        const auto &surrounding = inputContext->surroundingText();
        if (!surrounding.isValid()) {
            appendDebugMarker("selection-edit skipped invalid-surrounding");
            return std::nullopt;
        }

        const auto selectedText = selectionTextFromSurrounding(surrounding);
        if (selectedText.empty()) {
            appendDebugMarker("selection-edit skipped empty-selection anchor=" +
                              std::to_string(surrounding.anchor()) +
                              " cursor=" +
                              std::to_string(surrounding.cursor()));
            return std::nullopt;
        }

        SelectionEditTarget target;
        target.inputContext = inputContext->watch();
        target.anchor = surrounding.anchor();
        target.cursor = surrounding.cursor();
        target.selectedText = selectedText;
        appendDebugMarker("selection-edit captured bytes=" +
                          std::to_string(selectedText.size()));
        return target;
    }

    std::optional<SelectionEditTarget> cachedSelectionEditTarget(
        fcitx::InputContext *inputContext) const {
        if (!cachedSelectionEdit_) {
            return std::nullopt;
        }
        if (!cachedSelectionEdit_->inputContext.isValid()) {
            return std::nullopt;
        }
        if (cachedSelectionEdit_->inputContext.get() != inputContext) {
            return std::nullopt;
        }
        appendDebugMarker("selection-edit using cached target");
        return cachedSelectionEdit_;
    }

    void handleSurroundingTextUpdated(fcitx::Event &event) {
        auto &contextEvent = static_cast<fcitx::InputContextEvent &>(event);
        auto *inputContext = contextEvent.inputContext();
        if (!inputContext) {
            return;
        }

        auto target = captureSelectionEditTarget(inputContext);
        if (!target) {
            if (cachedSelectionEdit_ &&
                cachedSelectionEdit_->inputContext.get() == inputContext) {
                cachedSelectionEdit_.reset();
                appendDebugMarker("selection-edit cache cleared");
            }
            return;
        }

        cachedSelectionEdit_ = *target;
        appendDebugMarker("selection-edit cache updated bytes=" +
                          std::to_string(target->selectedText.size()));
    }

    void handleFocusOut(fcitx::Event &event) {
        auto &contextEvent = static_cast<fcitx::InputContextEvent &>(event);
        auto *inputContext = contextEvent.inputContext();
        if (!inputContext) {
            return;
        }

        triggerPressed_ = false;

        if (cachedSelectionEdit_ &&
            cachedSelectionEdit_->inputContext.get() == inputContext) {
            cachedSelectionEdit_.reset();
            appendDebugMarker("selection-edit cache cleared focus-out");
        }
    }

    void bindPendingSelectionEdit(const std::string &sessionID) {
        if (!pendingSelectionEdit_) {
            activeSelectionEdit_.reset();
            return;
        }

        activeSelectionEdit_ = ActiveSelectionEditTarget{
            sessionID,
            *pendingSelectionEdit_,
        };
        pendingSelectionEdit_.reset();
        appendDebugMarker("selection-edit bound session=" + sessionID);
    }

    void clearSelectionEditTargets() {
        pendingSelectionEdit_.reset();
        activeSelectionEdit_.reset();
    }

    bool replaceSelectionResult(const ActiveSelectionEditTarget &editTarget,
                                const std::string &text) {
        if (text.empty()) {
            appendDebugMarker("selection-edit replace skipped empty-text");
            return false;
        }

        auto *inputContext = editTarget.target.inputContext.get();
        if (!inputContext || !inputContext->hasFocus()) {
            appendDebugMarker("selection-edit replace failed no-target");
            return false;
        }

        const auto &surrounding = inputContext->surroundingText();
        if (!surrounding.isValid()) {
            appendDebugMarker("selection-edit replace failed invalid-surrounding");
            return false;
        }

        const auto expectedStart =
            std::min(editTarget.target.anchor, editTarget.target.cursor);
        const auto expectedEnd =
            std::max(editTarget.target.anchor, editTarget.target.cursor);
        const auto currentStart =
            std::min(surrounding.anchor(), surrounding.cursor());
        const auto currentEnd =
            std::max(surrounding.anchor(), surrounding.cursor());

        if (currentStart != expectedStart || currentEnd != expectedEnd) {
            appendDebugMarker("selection-edit replace failed selection-range-changed");
            return false;
        }
        if (surrounding.selectedText() != editTarget.target.selectedText) {
            appendDebugMarker("selection-edit replace failed selection-text-changed");
            return false;
        }

        const auto deleteSize = currentEnd - currentStart;
        if (deleteSize == 0) {
            appendDebugMarker("selection-edit replace failed empty-selection");
            return false;
        }

        const auto cursor = static_cast<int>(surrounding.cursor());
        const auto offset = static_cast<int>(currentStart) - cursor;
        fcitx::InputContextEventBlocker blocker(inputContext);
        inputContext->deleteSurroundingText(offset, deleteSize);
        inputContext->surroundingText().deleteText(offset, deleteSize);
        inputContext->commitString(text);
        appendDebugMarker("selection-edit replace ok bytes=" +
                          std::to_string(text.size()));
        return true;
    }

    void commitResult(const std::string &sessionID, const std::string &text) {
        if (text.empty()) {
            FCITX_WARN() << "coe-fcitx: empty result text";
            appendDebugMarker("commit skipped empty-text");
            return;
        }

        if (activeSelectionEdit_ &&
            activeSelectionEdit_->sessionID == sessionID) {
            const auto replaced =
                replaceSelectionResult(*activeSelectionEdit_, text);
            activeSelectionEdit_.reset();
            if (!replaced) {
                FCITX_WARN() << "coe-fcitx: selection edit validation failed";
                showErrorPanel("selection edit target changed");
                return;
            }

            FCITX_INFO() << "coe-fcitx: replaced selected text with "
                         << text.size() << " bytes";
            return;
        }
        if (activeSelectionEdit_ &&
            activeSelectionEdit_->sessionID != sessionID) {
            appendDebugMarker("selection-edit cleared stale session");
            activeSelectionEdit_.reset();
        }

        FCITX_DEBUG() << "coe-fcitx: attempting to commit " << text.size()
                      << " bytes to current input context";
        auto *inputContext = instance_->lastFocusedInputContext();
        if (!inputContext || !inputContext->hasFocus()) {
            FCITX_WARN() << "coe-fcitx: no focused input context for result";
            appendDebugMarker("commit skipped no-focused-input-context");
            return;
        }

        inputContext->commitString(text);
        FCITX_INFO() << "coe-fcitx: committed " << text.size()
                     << " bytes to current input context";
        appendDebugMarker("commit ok bytes=" + std::to_string(text.size()));
    }

    void handleKeyEvent(fcitx::Event &event) {
        auto &keyEvent = static_cast<fcitx::KeyEvent &>(event);
        if (handleEscapePress(keyEvent)) {
            return;
        }
        if (!keyEvent.key().check(triggerKey_)) {
            return;
        }

        if (keyEvent.isRelease()) {
            handleTriggerRelease(keyEvent);
            return;
        }

        handleTriggerPress(keyEvent);
    }

    bool handleEscapePress(fcitx::KeyEvent &keyEvent) {
        if (keyEvent.isRelease()) {
            return false;
        }
        if (!keyEvent.inputContext()) {
            return false;
        }
        if (!keyEvent.inputContext()->hasFocus()) {
            return false;
        }

        const auto escapeKey = fcitx::Key(kEscapeKey).normalize();
        if (!keyEvent.key().check(escapeKey)) {
            return false;
        }
        if (dictationState_ != "recording") {
            return false;
        }

        FCITX_DEBUG() << "coe-fcitx: Escape pressed during recording";
        appendDebugMarker("escape stop state=" + dictationState_);
        holding_ = false;
        if (!callCancel()) {
            FCITX_WARN() << "coe-fcitx: failed to call Coe Cancel() over D-Bus";
            appendDebugMarker("escape cancel failed");
            return false;
        }

        keyEvent.filterAndAccept();
        return true;
    }

    void handleTriggerPress(fcitx::KeyEvent &keyEvent) {
        if (!keyEvent.inputContext()) {
            return;
        }
        if (!keyEvent.inputContext()->hasFocus()) {
            return;
        }
        if (triggerPressed_) {
            appendDebugMarker("trigger press ignored repeat key=" +
                              triggerKey_.toString());
            keyEvent.filterAndAccept();
            return;
        }
        triggerPressed_ = true;

        FCITX_DEBUG() << "coe-fcitx: trigger matched for " << triggerKey_.toString();
        appendDebugMarker("trigger press key=" + triggerKey_.toString());
        auto editTarget = captureSelectionEditTarget(keyEvent.inputContext());
        if (!editTarget) {
            editTarget = cachedSelectionEditTarget(keyEvent.inputContext());
        }

        if (triggerMode_ == "hold") {
            if (holding_) {
                appendDebugMarker("hold press ignored already-holding");
                keyEvent.filterAndAccept();
                return;
            }
            if (editTarget) {
                pendingSelectionEdit_ = editTarget;
                if (!callStartWithSelectionEdit(editTarget->selectedText)) {
                    pendingSelectionEdit_.reset();
                    FCITX_WARN() << "coe-fcitx: failed to call Coe StartWithSelectionEdit() over D-Bus";
                    appendDebugMarker("start-with-selection failed");
                    return;
                }
            } else {
                pendingSelectionEdit_.reset();
                if (!callStart()) {
                    FCITX_WARN() << "coe-fcitx: failed to call Coe Start() over D-Bus";
                    appendDebugMarker("start failed");
                    return;
                }
            }
            holding_ = true;
            keyEvent.filterAndAccept();
            return;
        }

        if (dictationState_ == "recording") {
            pendingSelectionEdit_.reset();
            if (!callToggle()) {
                FCITX_WARN() << "coe-fcitx: failed to call Coe Toggle() over D-Bus";
                appendDebugMarker("toggle failed");
                return;
            }
            keyEvent.filterAndAccept();
            return;
        }

        if (editTarget) {
            pendingSelectionEdit_ = editTarget;
            if (!callToggleWithSelectionEdit(editTarget->selectedText)) {
                pendingSelectionEdit_.reset();
                FCITX_WARN() << "coe-fcitx: failed to call Coe ToggleWithSelectionEdit() over D-Bus";
                appendDebugMarker("toggle-with-selection failed");
                return;
            }
        } else {
            pendingSelectionEdit_.reset();
            if (!callToggle()) {
                FCITX_WARN() << "coe-fcitx: failed to call Coe Toggle() over D-Bus";
                appendDebugMarker("toggle failed");
                return;
            }
        }

        keyEvent.filterAndAccept();
    }

    void handleTriggerRelease(fcitx::KeyEvent &keyEvent) {
        triggerPressed_ = false;
        if (triggerMode_ != "hold") {
            return;
        }
        if (!holding_) {
            return;
        }

        FCITX_DEBUG() << "coe-fcitx: trigger release matched for "
                      << triggerKey_.toString();
        appendDebugMarker("trigger release key=" + triggerKey_.toString() +
                          " state=" + dictationState_);
        holding_ = false;
        if (!callStop()) {
            FCITX_WARN() << "coe-fcitx: failed to call Coe Stop() over D-Bus";
            appendDebugMarker("stop failed");
            return;
        }

        keyEvent.filterAndAccept();
    }

    bool callVoidMethod(const char *methodName) {
        if (!callBus_) {
            connectCallBus();
            if (!callBus_) {
                return false;
            }
        }

        DBusMessage *message = dbus_message_new_method_call(
            kServiceName, kObjectPath, kInterfaceName, methodName);
        if (!message) {
            FCITX_ERROR() << "coe-fcitx: failed to allocate D-Bus message";
            return false;
        }

        DBusError err;
        dbus_error_init(&err);
        DBusMessage *reply = dbus_connection_send_with_reply_and_block(
            callBus_, message, 2000, &err);
        dbus_message_unref(message);

        if (dbus_error_is_set(&err)) {
            FCITX_WARN() << "coe-fcitx: " << methodName << "() failed: "
                         << err.name << " "
                         << err.message;
            dbus_error_free(&err);
            return false;
        }
        if (!reply) {
            FCITX_WARN() << "coe-fcitx: " << methodName << "() returned no reply";
            return false;
        }

        dbus_message_unref(reply);
        FCITX_DEBUG() << "coe-fcitx: " << methodName << "() completed successfully";
        appendDebugMarker(toLower(std::string(methodName)) + " ok");
        return true;
    }

    bool callVoidMethodWithString(const char *methodName,
                                  const std::string &value) {
        if (!callBus_) {
            connectCallBus();
            if (!callBus_) {
                return false;
            }
        }

        DBusMessage *message = dbus_message_new_method_call(
            kServiceName, kObjectPath, kInterfaceName, methodName);
        if (!message) {
            FCITX_ERROR() << "coe-fcitx: failed to allocate D-Bus message";
            return false;
        }

        const char *rawValue = value.c_str();
        if (!dbus_message_append_args(message, DBUS_TYPE_STRING, &rawValue,
                                      DBUS_TYPE_INVALID)) {
            dbus_message_unref(message);
            FCITX_ERROR() << "coe-fcitx: failed to append D-Bus string argument";
            return false;
        }

        DBusError err;
        dbus_error_init(&err);
        DBusMessage *reply = dbus_connection_send_with_reply_and_block(
            callBus_, message, 2000, &err);
        dbus_message_unref(message);

        if (dbus_error_is_set(&err)) {
            FCITX_WARN() << "coe-fcitx: " << methodName << "() failed: "
                         << err.name << " " << err.message;
            dbus_error_free(&err);
            return false;
        }
        if (!reply) {
            FCITX_WARN() << "coe-fcitx: " << methodName << "() returned no reply";
            return false;
        }

        dbus_message_unref(reply);
        FCITX_DEBUG() << "coe-fcitx: " << methodName
                      << "() completed successfully";
        appendDebugMarker(toLower(std::string(methodName)) + " ok bytes=" +
                          std::to_string(value.size()));
        return true;
    }

    bool callToggle() { return callVoidMethod("Toggle"); }

    bool callToggleWithSelectionEdit(const std::string &selectedText) {
        return callVoidMethodWithString("ToggleWithSelectionEdit", selectedText);
    }

    bool callStart() { return callVoidMethod("Start"); }

    bool callStartWithSelectionEdit(const std::string &selectedText) {
        return callVoidMethodWithString("StartWithSelectionEdit", selectedText);
    }

    bool callCancel() { return callVoidMethod("Cancel"); }

    bool callStop() { return callVoidMethod("Stop"); }

    fcitx::Instance *instance_;
    fcitx::Key triggerKey_;
    std::string triggerKeySource_;
    std::string triggerMode_;
    std::string dictationState_;
    bool holding_ = false;
    bool triggerPressed_ = false;
    fcitx::EventDispatcher dispatcher_;
    DBusConnection *callBus_ = nullptr;
    DBusConnection *signalBus_ = nullptr;
    std::unique_ptr<fcitx::HandlerTableEntry<fcitx::EventHandler>> keyWatcher_;
    std::unique_ptr<fcitx::HandlerTableEntry<fcitx::EventHandler>> surroundingWatcher_;
    std::unique_ptr<fcitx::HandlerTableEntry<fcitx::EventHandler>> focusOutWatcher_;
    std::unique_ptr<fcitx::EventSourceTime> animationTimer_;
    std::unique_ptr<fcitx::EventSourceTime> clearTimer_;
    std::thread signalThread_;
    std::atomic<bool> running_ = false;
    std::string animatedPanelState_;
    size_t panelFrame_ = 0;
    std::optional<SelectionEditTarget> cachedSelectionEdit_;
    std::optional<SelectionEditTarget> pendingSelectionEdit_;
    std::optional<ActiveSelectionEditTarget> activeSelectionEdit_;
};

class CoeModuleFactory final : public fcitx::AddonFactory {
public:
    fcitx::AddonInstance *create(fcitx::AddonManager *manager) override {
        return new CoeModule(manager ? manager->instance() : nullptr);
    }
};

} // namespace

FCITX_ADDON_FACTORY(CoeModuleFactory)
