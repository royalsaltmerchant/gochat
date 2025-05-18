export default class RTCConn {
  constructor(props) {
    this.audioCtx = props.audioCtx;
    this.room = props.room;
    this.userID = props.userID;

    this.sfuUrl = "ws://99.36.161.96:7000/ws";
    this.turnUrl = "turn:99.36.161.96:3478?transport=udp"; // replace with your actual TURN

    this.socketConn = null;
    this.pc = null;
    this.messageID = 1;

    this.pendingCandidates = [];
    this.remoteDescriptionSet = false;
  }

  async start() {
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    console.log("🎙️ Got local stream");

    this.pc = new RTCPeerConnection({
      iceServers: [
        {
          urls: this.turnUrl,
          username: "1747510821",
          credential: "9U9t8QqEdHbKF71Fv4sU9GmN0vw",
        },
        { urls: "stun:stun.l.google.com:19302" },
      ],
      iceTransportPolicy: "all", // use "relay" to test TURN-only mode
    });

    // const audio = document.createElement("audio");
    // audio.srcObject = stream;
    // audio.autoplay = true;
    // audio.controls = true;
    // audio.muted = false;
    // document.body.appendChild(audio);

    // 🎤 Add local tracks before offer
    stream.getTracks().forEach((track) => {
      console.log("🎙️ Adding track:", track.kind);
      this.pc.addTrack(track, stream);
    });

    // 🔊 ICE + Track handling
    this.pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.sendJSONRPC("trickle", { candidate: event.candidate });
      } else {
        console.log("📶 ICE gathering complete");
      }
    };

    this.pc.oniceconnectionstatechange = () => {
      console.log("🌐 ICE state:", this.pc.iceConnectionState);
      if (this.pc.iceConnectionState === "failed") {
        console.warn("❌ ICE connection failed");
      }
    };

    this.pc.ontrack = async (event) => {
      console.log("📥 ontrack triggered");

      // Prefer event.streams[0], but fallback to event.track
      const remoteStream =
        event.streams?.[0] instanceof MediaStream
          ? event.streams[0]
          : new MediaStream([event.track]);

      const track = remoteStream.getAudioTracks()[0];
      if (!track) {
        console.warn("⚠️ No audio track in remote stream");
        return;
      }

      console.log("🎧 Received audio track:", {
        id: track.id,
        enabled: track.enabled,
        muted: track.muted,
        readyState: track.readyState,
      });

      // Ensure AudioContext is ready
      if (!this.audioCtx || this.audioCtx.state === "closed") {
        this.audioCtx = new (window.AudioContext ||
          window.webkitAudioContext)();
        await this.audioCtx.resume();
        console.log("🔊 AudioContext resumed");
      }

      try {
        const source = this.audioCtx.createMediaStreamSource(remoteStream);
        const gainNode = this.audioCtx.createGain();
        gainNode.gain.value = 1.0; // Set volume to 100%

        source.connect(gainNode).connect(this.audioCtx.destination);
        console.log("🔈 Audio routed to speakers");
      } catch (err) {
        console.error("❌ Failed to route audio through AudioContext", err);
      }

      // Optional: add a visible <audio> element for manual verification
      const testAudio = document.createElement("audio");
      testAudio.srcObject = remoteStream;
      testAudio.autoplay = true;
      testAudio.controls = true;
      testAudio.muted = false;
      document.body.appendChild(testAudio);
    };

    // setInterval(() => {
    //   this.pc.getStats().then((stats) => {
    //     console.log(stats);
    //     stats.forEach((report) => {
    //       console.log(report);
    //       if (report.kind === "audio") {
    //       }
    //     });
    //   });
    // }, 1000);

    // 📡 WebSocket signaling
    this.socketConn = new WebSocket(this.sfuUrl);
    this.socketConn.onmessage = this.onmessage;

    this.socketConn.onopen = async () => {
      console.log("🔌 WebSocket connected");

      const offer = await this.pc.createOffer();
      await this.pc.setLocalDescription(offer);
      console.log("📤 Sending offer SDP");

      this.sendJSONRPC("join", {
        sid: this.room,
        uid: String(this.userID),
        offer: {
          type: offer.type,
          sdp: offer.sdp,
        },
      });
    };

    this.socketConn.onclose = (e) => {
      console.log("🔌 WebSocket closed:", e.reason);
    };

    this.socketConn.onerror = (err) => {
      console.error("🛑 WebSocket error:", err);
    };
  }

  onmessage = async (msgEvent) => {
    const msg = JSON.parse(msgEvent.data);
    console.log("📩 Got message:", msg);

    // 🧠 Renegotiation offer from SFU
    if (msg.method === "offer" && msg.params) {
      console.log("📡 Got renegotiation offer");

      await this.pc.setRemoteDescription(new RTCSessionDescription(msg.params));
      const answer = await this.pc.createAnswer();
      await this.pc.setLocalDescription(answer);

      this.sendJSONRPC("answer", {
        desc: {
          type: answer.type,
          sdp: answer.sdp,
        },
      });
    }

    // // ✅ Join response
    if (msg.id && msg.result?.type === "answer") {
      console.log("✅ Setting initial remote description");
      await this.pc.setRemoteDescription(new RTCSessionDescription(msg.result));
      this.remoteDescriptionSet = true;

      for (const candidate of this.pendingCandidates) {
        await this.pc.addIceCandidate(candidate);
      }
      this.pendingCandidates = [];
    }

    // // ❄️ Trickle ICE
    if (msg.method === "trickle" && msg.params?.candidate) {
      if (this.remoteDescriptionSet) {
        await this.pc.addIceCandidate(msg.params.candidate);
      } else {
        console.log("⏳ Queuing ICE candidate until remoteDescription is set");
        this.pendingCandidates.push(msg.params.candidate);
      }
    }

    // 👋 Peer left
    if (msg.method === "peer-leave" && msg.params?.uid) {
      console.log(`👋 Peer ${msg.params.uid} left the room`);
    }
  };

  sendJSONRPC(method, params) {
    const payload = {
      jsonrpc: "2.0",
      id: String(this.messageID++),
      method,
      params,
    };
    console.log("➡️ Sending JSON-RPC:", payload);

    if (this.socketConn?.readyState === WebSocket.OPEN) {
      this.socketConn.send(JSON.stringify(payload));
    }
  }

  close() {
    this.sendJSONRPC("leave", {
      sid: this.room,
      uid: String(this.userID),
    });

    if (this.pc) {
      this.pc.getSenders().forEach((sender) => this.pc.removeTrack(sender));
      this.pc.close();
      this.pc = null;
    }

    if (this.socketConn && this.socketConn.readyState <= WebSocket.OPEN) {
      this.socketConn.close();
      this.socketConn = null;
    }
  }
}
