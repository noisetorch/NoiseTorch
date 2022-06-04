package pulseaudio

type command uint32

//go:generate stringer -type=command
const (
	/* Generic commands */
	commandError command = iota
	commandTimeout
	commandReply // 2

	/* CLIENT->SERVER */
	commandCreatePlaybackStream // 3
	commandDeletePlaybackStream
	commandCreateRecordStream
	commandDeleteRecordStream
	commandExit
	commandAuth // 8
	commandSetClientName
	commandLookupSink
	commandLookupSource
	commandDrainPlaybackStream
	commandStat
	commandGetPlaybackLatency
	commandCreateUploadStream
	commandDeleteUploadStream
	commandFinishUploadStream
	commandPlaySample
	commandRemoveSample // 19

	commandGetServerInfo
	commandGetSinkInfo
	commandGetSinkInfoList
	commandGetSourceInfo
	commandGetSourceInfoList
	commandGetModuleInfo
	commandGetModuleInfoList
	commandGetClientInfo
	commandGetClientInfoList
	commandGetSinkInputInfo
	commandGetSinkInputInfoList
	commandGetSourceOutputInfo
	commandGetSourceOutputInfoList
	commandGetSampleInfo
	commandGetSampleInfoList
	commandSubscribe

	commandSetSinkVolume
	commandSetSinkInputVolume
	commandSetSourceVolume

	commandSetSinkMute
	commandSetSourceMute // 40

	commandCorkPlaybackStream
	commandFlushPlaybackStream
	commandTriggerPlaybackStream // 43

	commandSetDefaultSink
	commandSetDefaultSource // 45

	commandSetPlaybackStreamName
	commandSetRecordStreamName // 47

	commandKillClient
	commandKillSinkInput
	commandKillSourceOutput // 50

	commandLoadModule
	commandUnloadModule // 52

	commandAddAutoloadObsolete
	commandRemoveAutoloadObsolete
	commandGetAutoloadInfoObsolete
	commandGetAutoloadInfoListObsolete //56

	commandGetRecordLatency
	commandCorkRecordStream
	commandFlushRecordStream
	commandPrebufPlaybackStream // 60

	/* SERVER->CLIENT */
	commandRequest // 61
	commandOverflow
	commandUnderflow
	commandPlaybackStreamKilled
	commandRecordStreamKilled
	commandSubscribeEvent

	/* A few more client->server commands */

	commandMoveSinkInput
	commandMoveSourceOutput
	commandSetSinkInputMute
	commandSuspendSink
	commandSuspendSource

	commandSetPlaybackStreamBufferAttr
	commandSetRecordStreamBufferAttr

	commandUpdatePlaybackStreamSampleRate
	commandUpdateRecordStreamSampleRate

	/* SERVER->CLIENT */
	commandPlaybackStreamSuspended
	commandRecordStreamSuspended
	commandPlaybackStreamMoved
	commandRecordStreamMoved

	commandUpdateRecordStreamProplist
	commandUpdatePlaybackStreamProplist
	commandUpdateClientProplist
	commandRemoveRecordStreamProplist
	commandRemovePlaybackStreamProplist
	commandRemoveClientProplist

	/* SERVER->CLIENT */
	commandStarted

	commandExtension

	commandGetCardInfo
	commandGetCardInfoList
	commandSetCardProfile

	commandClientEvent
	commandPlaybackStreamEvent
	commandRecordStreamEvent

	/* SERVER->CLIENT */
	commandPlaybackBufferAttrChanged
	commandRecordBufferAttrChanged

	commandSetSinkPort
	commandSetSourcePort

	commandSetSourceOutputVolume
	commandSetSourceOutputMute

	commandSetPortLatencyOffset

	/* BOTH DIRECTIONS */
	commandEnableSrbchannel
	commandDisableSrbchannel

	/* BOTH DIRECTIONS */
	commandRegisterMemfdShmid

	commandMax
)
