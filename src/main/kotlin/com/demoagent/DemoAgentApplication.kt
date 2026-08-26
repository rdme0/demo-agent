package com.demoagent

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication

@SpringBootApplication
class DemoAgentApplication

fun main(args: Array<String>) {
    runApplication<DemoAgentApplication>(*args)
}
