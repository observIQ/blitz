# bash completion for blitz                                -*- shell-script -*-

__blitz_debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE:-} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

# Homebrew on Macs have version 1.3 of bash-completion which doesn't include
# _init_completion. This is a very minimal version of that function.
__blitz_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}

__blitz_index_of_word()
{
    local w word=$1
    shift
    index=0
    for w in "$@"; do
        [[ $w = "$word" ]] && return
        index=$((index+1))
    done
    index=-1
}

__blitz_contains_word()
{
    local w word=$1; shift
    for w in "$@"; do
        [[ $w = "$word" ]] && return
    done
    return 1
}

__blitz_handle_go_custom_completion()
{
    __blitz_debug "${FUNCNAME[0]}: cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}"

    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16

    local out requestComp lastParam lastChar comp directive args

    # Prepare the command to request completions for the program.
    # Calling ${words[0]} instead of directly blitz allows handling aliases
    args=("${words[@]:1}")
    # Disable ActiveHelp which is not supported for bash completion v1
    requestComp="BLITZ_ACTIVE_HELP=0 ${words[0]} __completeNoDesc ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __blitz_debug "${FUNCNAME[0]}: lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __blitz_debug "${FUNCNAME[0]}: Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    __blitz_debug "${FUNCNAME[0]}: calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval "${requestComp}" 2>/dev/null)

    # Extract the directive integer at the very end of the output following a colon (:)
    directive=${out##*:}
    # Remove the directive
    out=${out%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __blitz_debug "${FUNCNAME[0]}: the completion directive is: ${directive}"
    __blitz_debug "${FUNCNAME[0]}: the completions are: ${out}"

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        # Error code.  No completion.
        __blitz_debug "${FUNCNAME[0]}: received error from custom completion go code"
        return
    else
        if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __blitz_debug "${FUNCNAME[0]}: activating no space"
                compopt -o nospace
            fi
        fi
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __blitz_debug "${FUNCNAME[0]}: activating no file completion"
                compopt +o default
            fi
        fi
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        # File extension filtering
        local fullFilter filter filteringCmd
        # Do not use quotes around the $out variable or else newline
        # characters will be kept.
        for filter in ${out}; do
            fullFilter+="$filter|"
        done

        filteringCmd="_filedir $fullFilter"
        __blitz_debug "File filtering command: $filteringCmd"
        $filteringCmd
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only
        local subdir
        # Use printf to strip any trailing newline
        subdir=$(printf "%s" "${out}")
        if [ -n "$subdir" ]; then
            __blitz_debug "Listing directories in $subdir"
            __blitz_handle_subdirs_in_dir_flag "$subdir"
        else
            __blitz_debug "Listing directories in ."
            _filedir -d
        fi
    else
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${out}" -- "$cur")
    fi
}

__blitz_handle_reply()
{
    __blitz_debug "${FUNCNAME[0]}"
    local comp
    case $cur in
        -*)
            if [[ $(type -t compopt) = "builtin" ]]; then
                compopt -o nospace
            fi
            local allflags
            if [ ${#must_have_one_flag[@]} -ne 0 ]; then
                allflags=("${must_have_one_flag[@]}")
            else
                allflags=("${flags[*]} ${two_word_flags[*]}")
            fi
            while IFS='' read -r comp; do
                COMPREPLY+=("$comp")
            done < <(compgen -W "${allflags[*]}" -- "$cur")
            if [[ $(type -t compopt) = "builtin" ]]; then
                [[ "${COMPREPLY[0]}" == *= ]] || compopt +o nospace
            fi

            # complete after --flag=abc
            if [[ $cur == *=* ]]; then
                if [[ $(type -t compopt) = "builtin" ]]; then
                    compopt +o nospace
                fi

                local index flag
                flag="${cur%=*}"
                __blitz_index_of_word "${flag}" "${flags_with_completion[@]}"
                COMPREPLY=()
                if [[ ${index} -ge 0 ]]; then
                    PREFIX=""
                    cur="${cur#*=}"
                    ${flags_completion[${index}]}
                    if [ -n "${ZSH_VERSION:-}" ]; then
                        # zsh completion needs --flag= prefix
                        eval "COMPREPLY=( \"\${COMPREPLY[@]/#/${flag}=}\" )"
                    fi
                fi
            fi

            if [[ -z "${flag_parsing_disabled}" ]]; then
                # If flag parsing is enabled, we have completed the flags and can return.
                # If flag parsing is disabled, we may not know all (or any) of the flags, so we fallthrough
                # to possibly call handle_go_custom_completion.
                return 0;
            fi
            ;;
    esac

    # check if we are handling a flag with special work handling
    local index
    __blitz_index_of_word "${prev}" "${flags_with_completion[@]}"
    if [[ ${index} -ge 0 ]]; then
        ${flags_completion[${index}]}
        return
    fi

    # we are parsing a flag and don't have a special handler, no completion
    if [[ ${cur} != "${words[cword]}" ]]; then
        return
    fi

    local completions
    completions=("${commands[@]}")
    if [[ ${#must_have_one_noun[@]} -ne 0 ]]; then
        completions+=("${must_have_one_noun[@]}")
    elif [[ -n "${has_completion_function}" ]]; then
        # if a go completion function is provided, defer to that function
        __blitz_handle_go_custom_completion
    fi
    if [[ ${#must_have_one_flag[@]} -ne 0 ]]; then
        completions+=("${must_have_one_flag[@]}")
    fi
    while IFS='' read -r comp; do
        COMPREPLY+=("$comp")
    done < <(compgen -W "${completions[*]}" -- "$cur")

    if [[ ${#COMPREPLY[@]} -eq 0 && ${#noun_aliases[@]} -gt 0 && ${#must_have_one_noun[@]} -ne 0 ]]; then
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${noun_aliases[*]}" -- "$cur")
    fi

    if [[ ${#COMPREPLY[@]} -eq 0 ]]; then
        if declare -F __blitz_custom_func >/dev/null; then
            # try command name qualified custom func
            __blitz_custom_func
        else
            # otherwise fall back to unqualified for compatibility
            declare -F __custom_func >/dev/null && __custom_func
        fi
    fi

    # available in bash-completion >= 2, not always present on macOS
    if declare -F __ltrim_colon_completions >/dev/null; then
        __ltrim_colon_completions "$cur"
    fi

    # If there is only 1 completion and it is a flag with an = it will be completed
    # but we don't want a space after the =
    if [[ "${#COMPREPLY[@]}" -eq "1" ]] && [[ $(type -t compopt) = "builtin" ]] && [[ "${COMPREPLY[0]}" == --*= ]]; then
       compopt -o nospace
    fi
}

# The arguments should be in the form "ext1|ext2|extn"
__blitz_handle_filename_extension_flag()
{
    local ext="$1"
    _filedir "@(${ext})"
}

__blitz_handle_subdirs_in_dir_flag()
{
    local dir="$1"
    pushd "${dir}" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1 || return
}

__blitz_handle_flag()
{
    __blitz_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    # if a command required a flag, and we found it, unset must_have_one_flag()
    local flagname=${words[c]}
    local flagvalue=""
    # if the word contained an =
    if [[ ${words[c]} == *"="* ]]; then
        flagvalue=${flagname#*=} # take in as flagvalue after the =
        flagname=${flagname%=*} # strip everything after the =
        flagname="${flagname}=" # but put the = back
    fi
    __blitz_debug "${FUNCNAME[0]}: looking for ${flagname}"
    if __blitz_contains_word "${flagname}" "${must_have_one_flag[@]}"; then
        must_have_one_flag=()
    fi

    # if you set a flag which only applies to this command, don't show subcommands
    if __blitz_contains_word "${flagname}" "${local_nonpersistent_flags[@]}"; then
      commands=()
    fi

    # keep flag value with flagname as flaghash
    # flaghash variable is an associative array which is only supported in bash > 3.
    if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
        if [ -n "${flagvalue}" ] ; then
            flaghash[${flagname}]=${flagvalue}
        elif [ -n "${words[ $((c+1)) ]}" ] ; then
            flaghash[${flagname}]=${words[ $((c+1)) ]}
        else
            flaghash[${flagname}]="true" # pad "true" for bool flag
        fi
    fi

    # skip the argument to a two word flag
    if [[ ${words[c]} != *"="* ]] && __blitz_contains_word "${words[c]}" "${two_word_flags[@]}"; then
        __blitz_debug "${FUNCNAME[0]}: found a flag ${words[c]}, skip the next argument"
        c=$((c+1))
        # if we are looking for a flags value, don't show commands
        if [[ $c -eq $cword ]]; then
            commands=()
        fi
    fi

    c=$((c+1))

}

__blitz_handle_noun()
{
    __blitz_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    if __blitz_contains_word "${words[c]}" "${must_have_one_noun[@]}"; then
        must_have_one_noun=()
    elif __blitz_contains_word "${words[c]}" "${noun_aliases[@]}"; then
        must_have_one_noun=()
    fi

    nouns+=("${words[c]}")
    c=$((c+1))
}

__blitz_handle_command()
{
    __blitz_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    local next_command
    if [[ -n ${last_command} ]]; then
        next_command="_${last_command}_${words[c]//:/__}"
    else
        if [[ $c -eq 0 ]]; then
            next_command="_blitz_root_command"
        else
            next_command="_${words[c]//:/__}"
        fi
    fi
    c=$((c+1))
    __blitz_debug "${FUNCNAME[0]}: looking for ${next_command}"
    declare -F "$next_command" >/dev/null && $next_command
}

__blitz_handle_word()
{
    if [[ $c -ge $cword ]]; then
        __blitz_handle_reply
        return
    fi
    __blitz_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __blitz_handle_flag
    elif __blitz_contains_word "${words[c]}" "${commands[@]}"; then
        __blitz_handle_command
    elif [[ $c -eq 0 ]]; then
        __blitz_handle_command
    elif __blitz_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __blitz_handle_command
        else
            __blitz_handle_noun
        fi
    else
        __blitz_handle_noun
    fi
    __blitz_handle_word
}

_blitz_help()
{
    last_command="blitz_help"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--generator-apache-combined-rate=")
    two_word_flags+=("--generator-apache-combined-rate")
    flags+=("--generator-apache-combined-workers=")
    two_word_flags+=("--generator-apache-combined-workers")
    flags+=("--generator-apache-common-rate=")
    two_word_flags+=("--generator-apache-common-rate")
    flags+=("--generator-apache-common-workers=")
    two_word_flags+=("--generator-apache-common-workers")
    flags+=("--generator-apache-error-rate=")
    two_word_flags+=("--generator-apache-error-rate")
    flags+=("--generator-apache-error-workers=")
    two_word_flags+=("--generator-apache-error-workers")
    flags+=("--generator-count=")
    two_word_flags+=("--generator-count")
    flags+=("--generator-filegen-cache-enabled")
    flags+=("--generator-filegen-cache-ttl=")
    two_word_flags+=("--generator-filegen-cache-ttl")
    flags+=("--generator-filegen-rate=")
    two_word_flags+=("--generator-filegen-rate")
    flags+=("--generator-filegen-source=")
    two_word_flags+=("--generator-filegen-source")
    flags+=("--generator-filegen-workers=")
    two_word_flags+=("--generator-filegen-workers")
    flags+=("--generator-hostmetrics-hostname=")
    two_word_flags+=("--generator-hostmetrics-hostname")
    flags+=("--generator-hostmetrics-os=")
    two_word_flags+=("--generator-hostmetrics-os")
    flags+=("--generator-hostmetrics-rate=")
    two_word_flags+=("--generator-hostmetrics-rate")
    flags+=("--generator-hostmetrics-scrapers=")
    two_word_flags+=("--generator-hostmetrics-scrapers")
    flags+=("--generator-hostmetrics-workers=")
    two_word_flags+=("--generator-hostmetrics-workers")
    flags+=("--generator-json-rate=")
    two_word_flags+=("--generator-json-rate")
    flags+=("--generator-json-type=")
    two_word_flags+=("--generator-json-type")
    flags_with_completion+=("--generator-json-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-json-workers=")
    two_word_flags+=("--generator-json-workers")
    flags+=("--generator-kubernetes-format=")
    two_word_flags+=("--generator-kubernetes-format")
    flags_with_completion+=("--generator-kubernetes-format")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-kubernetes-rate=")
    two_word_flags+=("--generator-kubernetes-rate")
    flags+=("--generator-kubernetes-workers=")
    two_word_flags+=("--generator-kubernetes-workers")
    flags+=("--generator-nginx-rate=")
    two_word_flags+=("--generator-nginx-rate")
    flags+=("--generator-nginx-workers=")
    two_word_flags+=("--generator-nginx-workers")
    flags+=("--generator-okta-rate=")
    two_word_flags+=("--generator-okta-rate")
    flags+=("--generator-okta-workers=")
    two_word_flags+=("--generator-okta-workers")
    flags+=("--generator-paloalto-rate=")
    two_word_flags+=("--generator-paloalto-rate")
    flags+=("--generator-paloalto-workers=")
    two_word_flags+=("--generator-paloalto-workers")
    flags+=("--generator-postgres-rate=")
    two_word_flags+=("--generator-postgres-rate")
    flags+=("--generator-postgres-workers=")
    two_word_flags+=("--generator-postgres-workers")
    flags+=("--generator-traces-rate=")
    two_word_flags+=("--generator-traces-rate")
    flags+=("--generator-traces-workers=")
    two_word_flags+=("--generator-traces-workers")
    flags+=("--generator-type=")
    two_word_flags+=("--generator-type")
    flags_with_completion+=("--generator-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-wel-channels=")
    two_word_flags+=("--generator-wel-channels")
    flags+=("--generator-wel-computer=")
    two_word_flags+=("--generator-wel-computer")
    flags+=("--generator-wel-domain=")
    two_word_flags+=("--generator-wel-domain")
    flags+=("--generator-wel-manageeventsources")
    flags+=("--generator-wel-rate=")
    two_word_flags+=("--generator-wel-rate")
    flags+=("--generator-wel-role=")
    two_word_flags+=("--generator-wel-role")
    flags+=("--generator-wel-workers=")
    two_word_flags+=("--generator-wel-workers")
    flags+=("--generator-winevt-rate=")
    two_word_flags+=("--generator-winevt-rate")
    flags+=("--generator-winevt-workers=")
    two_word_flags+=("--generator-winevt-workers")
    flags+=("--logging-file-path=")
    two_word_flags+=("--logging-file-path")
    flags+=("--logging-file-rotation-compress")
    flags+=("--logging-file-rotation-localtime")
    flags+=("--logging-file-rotation-maxagedays=")
    two_word_flags+=("--logging-file-rotation-maxagedays")
    flags+=("--logging-file-rotation-maxbackups=")
    two_word_flags+=("--logging-file-rotation-maxbackups")
    flags+=("--logging-file-rotation-maxsizemb=")
    two_word_flags+=("--logging-file-rotation-maxsizemb")
    flags+=("--logging-level=")
    two_word_flags+=("--logging-level")
    flags_with_completion+=("--logging-level")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--logging-type=")
    two_word_flags+=("--logging-type")
    flags_with_completion+=("--logging-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--metrics-port=")
    two_word_flags+=("--metrics-port")
    flags+=("--onfinish=")
    two_word_flags+=("--onfinish")
    flags+=("--otlp-grpc-tls-ca=")
    two_word_flags+=("--otlp-grpc-tls-ca")
    flags+=("--otlp-grpc-tls-cert=")
    two_word_flags+=("--otlp-grpc-tls-cert")
    flags+=("--otlp-grpc-tls-insecure")
    flags+=("--otlp-grpc-tls-key=")
    two_word_flags+=("--otlp-grpc-tls-key")
    flags+=("--otlp-grpc-tls-min-version=")
    two_word_flags+=("--otlp-grpc-tls-min-version")
    flags_with_completion+=("--otlp-grpc-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--otlp-grpc-tls-skip-verify")
    flags+=("--output-file-path=")
    two_word_flags+=("--output-file-path")
    flags+=("--output-file-rotation-compress")
    flags+=("--output-file-rotation-localtime")
    flags+=("--output-file-rotation-maxagedays=")
    two_word_flags+=("--output-file-rotation-maxagedays")
    flags+=("--output-file-rotation-maxbackups=")
    two_word_flags+=("--output-file-rotation-maxbackups")
    flags+=("--output-file-rotation-maxsizemb=")
    two_word_flags+=("--output-file-rotation-maxsizemb")
    flags+=("--output-file-workers=")
    two_word_flags+=("--output-file-workers")
    flags+=("--output-hec-ackpollinterval=")
    two_word_flags+=("--output-hec-ackpollinterval")
    flags+=("--output-hec-acktimeout=")
    two_word_flags+=("--output-hec-acktimeout")
    flags+=("--output-hec-batchsize=")
    two_word_flags+=("--output-hec-batchsize")
    flags+=("--output-hec-batchtimeout=")
    two_word_flags+=("--output-hec-batchtimeout")
    flags+=("--output-hec-enable-tls")
    flags+=("--output-hec-enableack")
    flags+=("--output-hec-eventformat=")
    two_word_flags+=("--output-hec-eventformat")
    flags+=("--output-hec-host=")
    two_word_flags+=("--output-hec-host")
    flags+=("--output-hec-index=")
    two_word_flags+=("--output-hec-index")
    flags+=("--output-hec-maxretries=")
    two_word_flags+=("--output-hec-maxretries")
    flags+=("--output-hec-port=")
    two_word_flags+=("--output-hec-port")
    flags+=("--output-hec-source=")
    two_word_flags+=("--output-hec-source")
    flags+=("--output-hec-sourcetype=")
    two_word_flags+=("--output-hec-sourcetype")
    flags+=("--output-hec-tls-ca=")
    two_word_flags+=("--output-hec-tls-ca")
    flags+=("--output-hec-tls-cert=")
    two_word_flags+=("--output-hec-tls-cert")
    flags+=("--output-hec-tls-key=")
    two_word_flags+=("--output-hec-tls-key")
    flags+=("--output-hec-tls-min-version=")
    two_word_flags+=("--output-hec-tls-min-version")
    flags+=("--output-hec-tls-skip-verify")
    flags+=("--output-hec-token=")
    two_word_flags+=("--output-hec-token")
    flags+=("--output-hec-workers=")
    two_word_flags+=("--output-hec-workers")
    flags+=("--output-otlpgrpc-batchtimeout=")
    two_word_flags+=("--output-otlpgrpc-batchtimeout")
    flags+=("--output-otlpgrpc-enable-tls")
    flags+=("--output-otlpgrpc-host=")
    two_word_flags+=("--output-otlpgrpc-host")
    flags+=("--output-otlpgrpc-maxexportbatchsize=")
    two_word_flags+=("--output-otlpgrpc-maxexportbatchsize")
    flags+=("--output-otlpgrpc-maxqueuesize=")
    two_word_flags+=("--output-otlpgrpc-maxqueuesize")
    flags+=("--output-otlpgrpc-port=")
    two_word_flags+=("--output-otlpgrpc-port")
    flags+=("--output-otlpgrpc-requesttimeout=")
    two_word_flags+=("--output-otlpgrpc-requesttimeout")
    flags+=("--output-otlpgrpc-workers=")
    two_word_flags+=("--output-otlpgrpc-workers")
    flags+=("--output-stdout-flushinterval=")
    two_word_flags+=("--output-stdout-flushinterval")
    flags+=("--output-syslog-appname=")
    two_word_flags+=("--output-syslog-appname")
    flags+=("--output-syslog-enable-tls")
    flags+=("--output-syslog-facility=")
    two_word_flags+=("--output-syslog-facility")
    flags+=("--output-syslog-host=")
    two_word_flags+=("--output-syslog-host")
    flags+=("--output-syslog-hostname=")
    two_word_flags+=("--output-syslog-hostname")
    flags+=("--output-syslog-maxdatagrambytes=")
    two_word_flags+=("--output-syslog-maxdatagrambytes")
    flags+=("--output-syslog-msgid=")
    two_word_flags+=("--output-syslog-msgid")
    flags+=("--output-syslog-port=")
    two_word_flags+=("--output-syslog-port")
    flags+=("--output-syslog-procid=")
    two_word_flags+=("--output-syslog-procid")
    flags+=("--output-syslog-rfc=")
    two_word_flags+=("--output-syslog-rfc")
    flags_with_completion+=("--output-syslog-rfc")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-ca=")
    two_word_flags+=("--output-syslog-tls-ca")
    flags+=("--output-syslog-tls-cert=")
    two_word_flags+=("--output-syslog-tls-cert")
    flags+=("--output-syslog-tls-key=")
    two_word_flags+=("--output-syslog-tls-key")
    flags+=("--output-syslog-tls-min-version=")
    two_word_flags+=("--output-syslog-tls-min-version")
    flags_with_completion+=("--output-syslog-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-skip-verify")
    flags+=("--output-syslog-transport=")
    two_word_flags+=("--output-syslog-transport")
    flags_with_completion+=("--output-syslog-transport")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-workers=")
    two_word_flags+=("--output-syslog-workers")
    flags+=("--output-tcp-enable-tls")
    flags+=("--output-tcp-host=")
    two_word_flags+=("--output-tcp-host")
    flags+=("--output-tcp-port=")
    two_word_flags+=("--output-tcp-port")
    flags+=("--output-tcp-tls-ca=")
    two_word_flags+=("--output-tcp-tls-ca")
    flags+=("--output-tcp-tls-cert=")
    two_word_flags+=("--output-tcp-tls-cert")
    flags+=("--output-tcp-tls-key=")
    two_word_flags+=("--output-tcp-tls-key")
    flags+=("--output-tcp-tls-min-version=")
    two_word_flags+=("--output-tcp-tls-min-version")
    flags_with_completion+=("--output-tcp-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-tcp-tls-skip-verify")
    flags+=("--output-tcp-workers=")
    two_word_flags+=("--output-tcp-workers")
    flags+=("--output-type=")
    two_word_flags+=("--output-type")
    flags_with_completion+=("--output-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-udp-host=")
    two_word_flags+=("--output-udp-host")
    flags+=("--output-udp-port=")
    two_word_flags+=("--output-udp-port")
    flags+=("--output-udp-workers=")
    two_word_flags+=("--output-udp-workers")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_blitz_version()
{
    last_command="blitz_version"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--generator-apache-combined-rate=")
    two_word_flags+=("--generator-apache-combined-rate")
    flags+=("--generator-apache-combined-workers=")
    two_word_flags+=("--generator-apache-combined-workers")
    flags+=("--generator-apache-common-rate=")
    two_word_flags+=("--generator-apache-common-rate")
    flags+=("--generator-apache-common-workers=")
    two_word_flags+=("--generator-apache-common-workers")
    flags+=("--generator-apache-error-rate=")
    two_word_flags+=("--generator-apache-error-rate")
    flags+=("--generator-apache-error-workers=")
    two_word_flags+=("--generator-apache-error-workers")
    flags+=("--generator-count=")
    two_word_flags+=("--generator-count")
    flags+=("--generator-filegen-cache-enabled")
    flags+=("--generator-filegen-cache-ttl=")
    two_word_flags+=("--generator-filegen-cache-ttl")
    flags+=("--generator-filegen-rate=")
    two_word_flags+=("--generator-filegen-rate")
    flags+=("--generator-filegen-source=")
    two_word_flags+=("--generator-filegen-source")
    flags+=("--generator-filegen-workers=")
    two_word_flags+=("--generator-filegen-workers")
    flags+=("--generator-hostmetrics-hostname=")
    two_word_flags+=("--generator-hostmetrics-hostname")
    flags+=("--generator-hostmetrics-os=")
    two_word_flags+=("--generator-hostmetrics-os")
    flags+=("--generator-hostmetrics-rate=")
    two_word_flags+=("--generator-hostmetrics-rate")
    flags+=("--generator-hostmetrics-scrapers=")
    two_word_flags+=("--generator-hostmetrics-scrapers")
    flags+=("--generator-hostmetrics-workers=")
    two_word_flags+=("--generator-hostmetrics-workers")
    flags+=("--generator-json-rate=")
    two_word_flags+=("--generator-json-rate")
    flags+=("--generator-json-type=")
    two_word_flags+=("--generator-json-type")
    flags_with_completion+=("--generator-json-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-json-workers=")
    two_word_flags+=("--generator-json-workers")
    flags+=("--generator-kubernetes-format=")
    two_word_flags+=("--generator-kubernetes-format")
    flags_with_completion+=("--generator-kubernetes-format")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-kubernetes-rate=")
    two_word_flags+=("--generator-kubernetes-rate")
    flags+=("--generator-kubernetes-workers=")
    two_word_flags+=("--generator-kubernetes-workers")
    flags+=("--generator-nginx-rate=")
    two_word_flags+=("--generator-nginx-rate")
    flags+=("--generator-nginx-workers=")
    two_word_flags+=("--generator-nginx-workers")
    flags+=("--generator-okta-rate=")
    two_word_flags+=("--generator-okta-rate")
    flags+=("--generator-okta-workers=")
    two_word_flags+=("--generator-okta-workers")
    flags+=("--generator-paloalto-rate=")
    two_word_flags+=("--generator-paloalto-rate")
    flags+=("--generator-paloalto-workers=")
    two_word_flags+=("--generator-paloalto-workers")
    flags+=("--generator-postgres-rate=")
    two_word_flags+=("--generator-postgres-rate")
    flags+=("--generator-postgres-workers=")
    two_word_flags+=("--generator-postgres-workers")
    flags+=("--generator-traces-rate=")
    two_word_flags+=("--generator-traces-rate")
    flags+=("--generator-traces-workers=")
    two_word_flags+=("--generator-traces-workers")
    flags+=("--generator-type=")
    two_word_flags+=("--generator-type")
    flags_with_completion+=("--generator-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-wel-channels=")
    two_word_flags+=("--generator-wel-channels")
    flags+=("--generator-wel-computer=")
    two_word_flags+=("--generator-wel-computer")
    flags+=("--generator-wel-domain=")
    two_word_flags+=("--generator-wel-domain")
    flags+=("--generator-wel-manageeventsources")
    flags+=("--generator-wel-rate=")
    two_word_flags+=("--generator-wel-rate")
    flags+=("--generator-wel-role=")
    two_word_flags+=("--generator-wel-role")
    flags+=("--generator-wel-workers=")
    two_word_flags+=("--generator-wel-workers")
    flags+=("--generator-winevt-rate=")
    two_word_flags+=("--generator-winevt-rate")
    flags+=("--generator-winevt-workers=")
    two_word_flags+=("--generator-winevt-workers")
    flags+=("--logging-file-path=")
    two_word_flags+=("--logging-file-path")
    flags+=("--logging-file-rotation-compress")
    flags+=("--logging-file-rotation-localtime")
    flags+=("--logging-file-rotation-maxagedays=")
    two_word_flags+=("--logging-file-rotation-maxagedays")
    flags+=("--logging-file-rotation-maxbackups=")
    two_word_flags+=("--logging-file-rotation-maxbackups")
    flags+=("--logging-file-rotation-maxsizemb=")
    two_word_flags+=("--logging-file-rotation-maxsizemb")
    flags+=("--logging-level=")
    two_word_flags+=("--logging-level")
    flags_with_completion+=("--logging-level")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--logging-type=")
    two_word_flags+=("--logging-type")
    flags_with_completion+=("--logging-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--metrics-port=")
    two_word_flags+=("--metrics-port")
    flags+=("--onfinish=")
    two_word_flags+=("--onfinish")
    flags+=("--otlp-grpc-tls-ca=")
    two_word_flags+=("--otlp-grpc-tls-ca")
    flags+=("--otlp-grpc-tls-cert=")
    two_word_flags+=("--otlp-grpc-tls-cert")
    flags+=("--otlp-grpc-tls-insecure")
    flags+=("--otlp-grpc-tls-key=")
    two_word_flags+=("--otlp-grpc-tls-key")
    flags+=("--otlp-grpc-tls-min-version=")
    two_word_flags+=("--otlp-grpc-tls-min-version")
    flags_with_completion+=("--otlp-grpc-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--otlp-grpc-tls-skip-verify")
    flags+=("--output-file-path=")
    two_word_flags+=("--output-file-path")
    flags+=("--output-file-rotation-compress")
    flags+=("--output-file-rotation-localtime")
    flags+=("--output-file-rotation-maxagedays=")
    two_word_flags+=("--output-file-rotation-maxagedays")
    flags+=("--output-file-rotation-maxbackups=")
    two_word_flags+=("--output-file-rotation-maxbackups")
    flags+=("--output-file-rotation-maxsizemb=")
    two_word_flags+=("--output-file-rotation-maxsizemb")
    flags+=("--output-file-workers=")
    two_word_flags+=("--output-file-workers")
    flags+=("--output-hec-ackpollinterval=")
    two_word_flags+=("--output-hec-ackpollinterval")
    flags+=("--output-hec-acktimeout=")
    two_word_flags+=("--output-hec-acktimeout")
    flags+=("--output-hec-batchsize=")
    two_word_flags+=("--output-hec-batchsize")
    flags+=("--output-hec-batchtimeout=")
    two_word_flags+=("--output-hec-batchtimeout")
    flags+=("--output-hec-enable-tls")
    flags+=("--output-hec-enableack")
    flags+=("--output-hec-eventformat=")
    two_word_flags+=("--output-hec-eventformat")
    flags+=("--output-hec-host=")
    two_word_flags+=("--output-hec-host")
    flags+=("--output-hec-index=")
    two_word_flags+=("--output-hec-index")
    flags+=("--output-hec-maxretries=")
    two_word_flags+=("--output-hec-maxretries")
    flags+=("--output-hec-port=")
    two_word_flags+=("--output-hec-port")
    flags+=("--output-hec-source=")
    two_word_flags+=("--output-hec-source")
    flags+=("--output-hec-sourcetype=")
    two_word_flags+=("--output-hec-sourcetype")
    flags+=("--output-hec-tls-ca=")
    two_word_flags+=("--output-hec-tls-ca")
    flags+=("--output-hec-tls-cert=")
    two_word_flags+=("--output-hec-tls-cert")
    flags+=("--output-hec-tls-key=")
    two_word_flags+=("--output-hec-tls-key")
    flags+=("--output-hec-tls-min-version=")
    two_word_flags+=("--output-hec-tls-min-version")
    flags+=("--output-hec-tls-skip-verify")
    flags+=("--output-hec-token=")
    two_word_flags+=("--output-hec-token")
    flags+=("--output-hec-workers=")
    two_word_flags+=("--output-hec-workers")
    flags+=("--output-otlpgrpc-batchtimeout=")
    two_word_flags+=("--output-otlpgrpc-batchtimeout")
    flags+=("--output-otlpgrpc-enable-tls")
    flags+=("--output-otlpgrpc-host=")
    two_word_flags+=("--output-otlpgrpc-host")
    flags+=("--output-otlpgrpc-maxexportbatchsize=")
    two_word_flags+=("--output-otlpgrpc-maxexportbatchsize")
    flags+=("--output-otlpgrpc-maxqueuesize=")
    two_word_flags+=("--output-otlpgrpc-maxqueuesize")
    flags+=("--output-otlpgrpc-port=")
    two_word_flags+=("--output-otlpgrpc-port")
    flags+=("--output-otlpgrpc-requesttimeout=")
    two_word_flags+=("--output-otlpgrpc-requesttimeout")
    flags+=("--output-otlpgrpc-workers=")
    two_word_flags+=("--output-otlpgrpc-workers")
    flags+=("--output-stdout-flushinterval=")
    two_word_flags+=("--output-stdout-flushinterval")
    flags+=("--output-syslog-appname=")
    two_word_flags+=("--output-syslog-appname")
    flags+=("--output-syslog-enable-tls")
    flags+=("--output-syslog-facility=")
    two_word_flags+=("--output-syslog-facility")
    flags+=("--output-syslog-host=")
    two_word_flags+=("--output-syslog-host")
    flags+=("--output-syslog-hostname=")
    two_word_flags+=("--output-syslog-hostname")
    flags+=("--output-syslog-maxdatagrambytes=")
    two_word_flags+=("--output-syslog-maxdatagrambytes")
    flags+=("--output-syslog-msgid=")
    two_word_flags+=("--output-syslog-msgid")
    flags+=("--output-syslog-port=")
    two_word_flags+=("--output-syslog-port")
    flags+=("--output-syslog-procid=")
    two_word_flags+=("--output-syslog-procid")
    flags+=("--output-syslog-rfc=")
    two_word_flags+=("--output-syslog-rfc")
    flags_with_completion+=("--output-syslog-rfc")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-ca=")
    two_word_flags+=("--output-syslog-tls-ca")
    flags+=("--output-syslog-tls-cert=")
    two_word_flags+=("--output-syslog-tls-cert")
    flags+=("--output-syslog-tls-key=")
    two_word_flags+=("--output-syslog-tls-key")
    flags+=("--output-syslog-tls-min-version=")
    two_word_flags+=("--output-syslog-tls-min-version")
    flags_with_completion+=("--output-syslog-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-skip-verify")
    flags+=("--output-syslog-transport=")
    two_word_flags+=("--output-syslog-transport")
    flags_with_completion+=("--output-syslog-transport")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-workers=")
    two_word_flags+=("--output-syslog-workers")
    flags+=("--output-tcp-enable-tls")
    flags+=("--output-tcp-host=")
    two_word_flags+=("--output-tcp-host")
    flags+=("--output-tcp-port=")
    two_word_flags+=("--output-tcp-port")
    flags+=("--output-tcp-tls-ca=")
    two_word_flags+=("--output-tcp-tls-ca")
    flags+=("--output-tcp-tls-cert=")
    two_word_flags+=("--output-tcp-tls-cert")
    flags+=("--output-tcp-tls-key=")
    two_word_flags+=("--output-tcp-tls-key")
    flags+=("--output-tcp-tls-min-version=")
    two_word_flags+=("--output-tcp-tls-min-version")
    flags_with_completion+=("--output-tcp-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-tcp-tls-skip-verify")
    flags+=("--output-tcp-workers=")
    two_word_flags+=("--output-tcp-workers")
    flags+=("--output-type=")
    two_word_flags+=("--output-type")
    flags_with_completion+=("--output-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-udp-host=")
    two_word_flags+=("--output-udp-host")
    flags+=("--output-udp-port=")
    two_word_flags+=("--output-udp-port")
    flags+=("--output-udp-workers=")
    two_word_flags+=("--output-udp-workers")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_blitz_root_command()
{
    last_command="blitz"

    command_aliases=()

    commands=()
    commands+=("help")
    commands+=("version")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--generator-apache-combined-rate=")
    two_word_flags+=("--generator-apache-combined-rate")
    flags+=("--generator-apache-combined-workers=")
    two_word_flags+=("--generator-apache-combined-workers")
    flags+=("--generator-apache-common-rate=")
    two_word_flags+=("--generator-apache-common-rate")
    flags+=("--generator-apache-common-workers=")
    two_word_flags+=("--generator-apache-common-workers")
    flags+=("--generator-apache-error-rate=")
    two_word_flags+=("--generator-apache-error-rate")
    flags+=("--generator-apache-error-workers=")
    two_word_flags+=("--generator-apache-error-workers")
    flags+=("--generator-count=")
    two_word_flags+=("--generator-count")
    flags+=("--generator-filegen-cache-enabled")
    flags+=("--generator-filegen-cache-ttl=")
    two_word_flags+=("--generator-filegen-cache-ttl")
    flags+=("--generator-filegen-rate=")
    two_word_flags+=("--generator-filegen-rate")
    flags+=("--generator-filegen-source=")
    two_word_flags+=("--generator-filegen-source")
    flags+=("--generator-filegen-workers=")
    two_word_flags+=("--generator-filegen-workers")
    flags+=("--generator-hostmetrics-hostname=")
    two_word_flags+=("--generator-hostmetrics-hostname")
    flags+=("--generator-hostmetrics-os=")
    two_word_flags+=("--generator-hostmetrics-os")
    flags+=("--generator-hostmetrics-rate=")
    two_word_flags+=("--generator-hostmetrics-rate")
    flags+=("--generator-hostmetrics-scrapers=")
    two_word_flags+=("--generator-hostmetrics-scrapers")
    flags+=("--generator-hostmetrics-workers=")
    two_word_flags+=("--generator-hostmetrics-workers")
    flags+=("--generator-json-rate=")
    two_word_flags+=("--generator-json-rate")
    flags+=("--generator-json-type=")
    two_word_flags+=("--generator-json-type")
    flags_with_completion+=("--generator-json-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-json-workers=")
    two_word_flags+=("--generator-json-workers")
    flags+=("--generator-kubernetes-format=")
    two_word_flags+=("--generator-kubernetes-format")
    flags_with_completion+=("--generator-kubernetes-format")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-kubernetes-rate=")
    two_word_flags+=("--generator-kubernetes-rate")
    flags+=("--generator-kubernetes-workers=")
    two_word_flags+=("--generator-kubernetes-workers")
    flags+=("--generator-nginx-rate=")
    two_word_flags+=("--generator-nginx-rate")
    flags+=("--generator-nginx-workers=")
    two_word_flags+=("--generator-nginx-workers")
    flags+=("--generator-okta-rate=")
    two_word_flags+=("--generator-okta-rate")
    flags+=("--generator-okta-workers=")
    two_word_flags+=("--generator-okta-workers")
    flags+=("--generator-paloalto-rate=")
    two_word_flags+=("--generator-paloalto-rate")
    flags+=("--generator-paloalto-workers=")
    two_word_flags+=("--generator-paloalto-workers")
    flags+=("--generator-postgres-rate=")
    two_word_flags+=("--generator-postgres-rate")
    flags+=("--generator-postgres-workers=")
    two_word_flags+=("--generator-postgres-workers")
    flags+=("--generator-traces-rate=")
    two_word_flags+=("--generator-traces-rate")
    flags+=("--generator-traces-workers=")
    two_word_flags+=("--generator-traces-workers")
    flags+=("--generator-type=")
    two_word_flags+=("--generator-type")
    flags_with_completion+=("--generator-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--generator-wel-channels=")
    two_word_flags+=("--generator-wel-channels")
    flags+=("--generator-wel-computer=")
    two_word_flags+=("--generator-wel-computer")
    flags+=("--generator-wel-domain=")
    two_word_flags+=("--generator-wel-domain")
    flags+=("--generator-wel-manageeventsources")
    flags+=("--generator-wel-rate=")
    two_word_flags+=("--generator-wel-rate")
    flags+=("--generator-wel-role=")
    two_word_flags+=("--generator-wel-role")
    flags+=("--generator-wel-workers=")
    two_word_flags+=("--generator-wel-workers")
    flags+=("--generator-winevt-rate=")
    two_word_flags+=("--generator-winevt-rate")
    flags+=("--generator-winevt-workers=")
    two_word_flags+=("--generator-winevt-workers")
    flags+=("--logging-file-path=")
    two_word_flags+=("--logging-file-path")
    flags+=("--logging-file-rotation-compress")
    flags+=("--logging-file-rotation-localtime")
    flags+=("--logging-file-rotation-maxagedays=")
    two_word_flags+=("--logging-file-rotation-maxagedays")
    flags+=("--logging-file-rotation-maxbackups=")
    two_word_flags+=("--logging-file-rotation-maxbackups")
    flags+=("--logging-file-rotation-maxsizemb=")
    two_word_flags+=("--logging-file-rotation-maxsizemb")
    flags+=("--logging-level=")
    two_word_flags+=("--logging-level")
    flags_with_completion+=("--logging-level")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--logging-type=")
    two_word_flags+=("--logging-type")
    flags_with_completion+=("--logging-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--metrics-port=")
    two_word_flags+=("--metrics-port")
    flags+=("--onfinish=")
    two_word_flags+=("--onfinish")
    flags+=("--otlp-grpc-tls-ca=")
    two_word_flags+=("--otlp-grpc-tls-ca")
    flags+=("--otlp-grpc-tls-cert=")
    two_word_flags+=("--otlp-grpc-tls-cert")
    flags+=("--otlp-grpc-tls-insecure")
    flags+=("--otlp-grpc-tls-key=")
    two_word_flags+=("--otlp-grpc-tls-key")
    flags+=("--otlp-grpc-tls-min-version=")
    two_word_flags+=("--otlp-grpc-tls-min-version")
    flags_with_completion+=("--otlp-grpc-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--otlp-grpc-tls-skip-verify")
    flags+=("--output-file-path=")
    two_word_flags+=("--output-file-path")
    flags+=("--output-file-rotation-compress")
    flags+=("--output-file-rotation-localtime")
    flags+=("--output-file-rotation-maxagedays=")
    two_word_flags+=("--output-file-rotation-maxagedays")
    flags+=("--output-file-rotation-maxbackups=")
    two_word_flags+=("--output-file-rotation-maxbackups")
    flags+=("--output-file-rotation-maxsizemb=")
    two_word_flags+=("--output-file-rotation-maxsizemb")
    flags+=("--output-file-workers=")
    two_word_flags+=("--output-file-workers")
    flags+=("--output-hec-ackpollinterval=")
    two_word_flags+=("--output-hec-ackpollinterval")
    flags+=("--output-hec-acktimeout=")
    two_word_flags+=("--output-hec-acktimeout")
    flags+=("--output-hec-batchsize=")
    two_word_flags+=("--output-hec-batchsize")
    flags+=("--output-hec-batchtimeout=")
    two_word_flags+=("--output-hec-batchtimeout")
    flags+=("--output-hec-enable-tls")
    flags+=("--output-hec-enableack")
    flags+=("--output-hec-eventformat=")
    two_word_flags+=("--output-hec-eventformat")
    flags+=("--output-hec-host=")
    two_word_flags+=("--output-hec-host")
    flags+=("--output-hec-index=")
    two_word_flags+=("--output-hec-index")
    flags+=("--output-hec-maxretries=")
    two_word_flags+=("--output-hec-maxretries")
    flags+=("--output-hec-port=")
    two_word_flags+=("--output-hec-port")
    flags+=("--output-hec-source=")
    two_word_flags+=("--output-hec-source")
    flags+=("--output-hec-sourcetype=")
    two_word_flags+=("--output-hec-sourcetype")
    flags+=("--output-hec-tls-ca=")
    two_word_flags+=("--output-hec-tls-ca")
    flags+=("--output-hec-tls-cert=")
    two_word_flags+=("--output-hec-tls-cert")
    flags+=("--output-hec-tls-key=")
    two_word_flags+=("--output-hec-tls-key")
    flags+=("--output-hec-tls-min-version=")
    two_word_flags+=("--output-hec-tls-min-version")
    flags+=("--output-hec-tls-skip-verify")
    flags+=("--output-hec-token=")
    two_word_flags+=("--output-hec-token")
    flags+=("--output-hec-workers=")
    two_word_flags+=("--output-hec-workers")
    flags+=("--output-otlpgrpc-batchtimeout=")
    two_word_flags+=("--output-otlpgrpc-batchtimeout")
    flags+=("--output-otlpgrpc-enable-tls")
    flags+=("--output-otlpgrpc-host=")
    two_word_flags+=("--output-otlpgrpc-host")
    flags+=("--output-otlpgrpc-maxexportbatchsize=")
    two_word_flags+=("--output-otlpgrpc-maxexportbatchsize")
    flags+=("--output-otlpgrpc-maxqueuesize=")
    two_word_flags+=("--output-otlpgrpc-maxqueuesize")
    flags+=("--output-otlpgrpc-port=")
    two_word_flags+=("--output-otlpgrpc-port")
    flags+=("--output-otlpgrpc-requesttimeout=")
    two_word_flags+=("--output-otlpgrpc-requesttimeout")
    flags+=("--output-otlpgrpc-workers=")
    two_word_flags+=("--output-otlpgrpc-workers")
    flags+=("--output-stdout-flushinterval=")
    two_word_flags+=("--output-stdout-flushinterval")
    flags+=("--output-syslog-appname=")
    two_word_flags+=("--output-syslog-appname")
    flags+=("--output-syslog-enable-tls")
    flags+=("--output-syslog-facility=")
    two_word_flags+=("--output-syslog-facility")
    flags+=("--output-syslog-host=")
    two_word_flags+=("--output-syslog-host")
    flags+=("--output-syslog-hostname=")
    two_word_flags+=("--output-syslog-hostname")
    flags+=("--output-syslog-maxdatagrambytes=")
    two_word_flags+=("--output-syslog-maxdatagrambytes")
    flags+=("--output-syslog-msgid=")
    two_word_flags+=("--output-syslog-msgid")
    flags+=("--output-syslog-port=")
    two_word_flags+=("--output-syslog-port")
    flags+=("--output-syslog-procid=")
    two_word_flags+=("--output-syslog-procid")
    flags+=("--output-syslog-rfc=")
    two_word_flags+=("--output-syslog-rfc")
    flags_with_completion+=("--output-syslog-rfc")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-ca=")
    two_word_flags+=("--output-syslog-tls-ca")
    flags+=("--output-syslog-tls-cert=")
    two_word_flags+=("--output-syslog-tls-cert")
    flags+=("--output-syslog-tls-key=")
    two_word_flags+=("--output-syslog-tls-key")
    flags+=("--output-syslog-tls-min-version=")
    two_word_flags+=("--output-syslog-tls-min-version")
    flags_with_completion+=("--output-syslog-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-tls-skip-verify")
    flags+=("--output-syslog-transport=")
    two_word_flags+=("--output-syslog-transport")
    flags_with_completion+=("--output-syslog-transport")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-syslog-workers=")
    two_word_flags+=("--output-syslog-workers")
    flags+=("--output-tcp-enable-tls")
    flags+=("--output-tcp-host=")
    two_word_flags+=("--output-tcp-host")
    flags+=("--output-tcp-port=")
    two_word_flags+=("--output-tcp-port")
    flags+=("--output-tcp-tls-ca=")
    two_word_flags+=("--output-tcp-tls-ca")
    flags+=("--output-tcp-tls-cert=")
    two_word_flags+=("--output-tcp-tls-cert")
    flags+=("--output-tcp-tls-key=")
    two_word_flags+=("--output-tcp-tls-key")
    flags+=("--output-tcp-tls-min-version=")
    two_word_flags+=("--output-tcp-tls-min-version")
    flags_with_completion+=("--output-tcp-tls-min-version")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-tcp-tls-skip-verify")
    flags+=("--output-tcp-workers=")
    two_word_flags+=("--output-tcp-workers")
    flags+=("--output-type=")
    two_word_flags+=("--output-type")
    flags_with_completion+=("--output-type")
    flags_completion+=("__blitz_handle_go_custom_completion")
    flags+=("--output-udp-host=")
    two_word_flags+=("--output-udp-host")
    flags+=("--output-udp-port=")
    two_word_flags+=("--output-udp-port")
    flags+=("--output-udp-workers=")
    two_word_flags+=("--output-udp-workers")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

__start_blitz()
{
    local cur prev words cword split
    declare -A flaghash 2>/dev/null || :
    declare -A aliashash 2>/dev/null || :
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -s || return
    else
        __blitz_init_completion -n "=" || return
    fi

    local c=0
    local flag_parsing_disabled=
    local flags=()
    local two_word_flags=()
    local local_nonpersistent_flags=()
    local flags_with_completion=()
    local flags_completion=()
    local commands=("blitz")
    local command_aliases=()
    local must_have_one_flag=()
    local must_have_one_noun=()
    local has_completion_function=""
    local last_command=""
    local nouns=()
    local noun_aliases=()

    __blitz_handle_word
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_blitz blitz
else
    complete -o default -o nospace -F __start_blitz blitz
fi

# ex: ts=4 sw=4 et filetype=sh
